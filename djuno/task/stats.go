package task

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/Djuno-Ltd/agent/setup"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

var arg = setup.GetArgs()

type Status struct {
	Id     string            `json:"id"`
	Disk   DiskStatus        `json:"disk"`
	Cpu    CpuStatus         `json:"cpu"`
	Memory MemoryStatus      `json:"memory"`
	Tasks  []ContainerStatus `json:"tasks"`
}

type DiskStatus struct {
	Total          uint64  `json:"total"`
	Used           uint64  `json:"used"`
	UsedPercentage float64 `json:"usedPercentage"`
	Free           uint64  `json:"free"`
}

type CpuStatus struct {
	UsedPercentage float64 `json:"usedPercentage"`
	Cores          int     `json:"cores"`
}

type MemoryStatus struct {
	Total          uint64  `json:"total"`
	Used           uint64  `json:"used"`
	UsedPercentage float64 `json:"usedPercentage"`
	Free           uint64  `json:"free"`
}

type ContainerStatus struct {
	Name             string  `json:"name"`
	ID               string  `json:"id"`
	CPUPercentage    float64 `json:"cpuPercentage"`
	Memory           float64 `json:"memory"`
	MemoryLimit      float64 `json:"memoryLimit"`
	MemoryPercentage float64 `json:"memoryPercentage"`
}

func getPath() string {
	runtimeOS := runtime.GOOS
	if runtimeOS == "windows" {
		return "\\"
	}
	return "/"
}

func DiskUsage() (ds DiskStatus) {
	diskStat, err := disk.Usage(getPath())
	if err != nil {
		return
	}

	ds.Total = diskStat.Total
	ds.Free = diskStat.Free
	ds.Used = diskStat.Used
	ds.UsedPercentage = diskStat.UsedPercent
	return
}

func CpuUsage(cpuCores int) (cs CpuStatus) {
	percentage, err := cpu.Percent(0, true)
	if err != nil {
		return
	}

	var cpuPercentAll, cpuPercentage float64
	for _, cpuPercent := range percentage {
		cpuPercentAll += cpuPercent
	}

	cpuPercentage = cpuPercentAll / (float64(len(percentage)))
	cs.UsedPercentage = cpuPercentage
	cs.Cores = cpuCores
	return
}

func MemoryUsage() (ms MemoryStatus) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return
	}

	ms.Total = vmStat.Total
	ms.Free = vmStat.Free
	ms.Used = vmStat.Used
	ms.UsedPercentage = vmStat.UsedPercent
	return
}

func ContainersUsage(cli *client.Client) (stats []ContainerStatus) {
	resp, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Printf("ERROR: Cannot obtain container list: %s\n", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(resp))
	mux := &sync.Mutex{}

	for _, v := range resp {
		go func(v types.Container) {
			defer wg.Done()
			var stat = ContainerUsage(cli, v.ID)
			mux.Lock()
			stats = append(stats, stat)
			mux.Unlock()
		}(v)
	}

	wg.Wait()
	return
}

func ContainerUsage(cli *client.Client, id string) (status ContainerStatus) {
	resp, err := cli.ContainerStats(context.Background(), id, false)
	if err != nil {
		log.Printf("ERROR: Statistics fetching failed: %s\n", err)
		return
	}
	var (
		v                         *types.StatsJSON
		previousCPU               uint64
		previousSystem            uint64
		memoryPercent, cpuPercent float64
		memory, memoryLimit       float64
	)

	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&v); err != nil {
		dec = json.NewDecoder(io.MultiReader(dec.Buffered(), resp.Body))
	}

	var daemonOSType = resp.OSType
	if daemonOSType != "windows" {
		previousCPU = v.PreCPUStats.CPUUsage.TotalUsage
		previousSystem = v.PreCPUStats.SystemUsage
		cpuPercent = calculateCPUPercentUnix(previousCPU, previousSystem, v)
		memory = calculateMemoryUsageUnixNoCache(v.MemoryStats)
		memoryLimit = float64(v.MemoryStats.Limit)
		memoryPercent = calculateMemoryPercentUnixNoCache(memoryLimit, memory)
	} else {
		cpuPercent = calculateCPUPercentWindows(v)
		memory = float64(v.MemoryStats.PrivateWorkingSet)
	}

	status.Name = v.Name
	status.ID = v.ID
	status.CPUPercentage = cpuPercent
	status.Memory = memory
	status.MemoryLimit = memoryLimit
	status.MemoryPercentage = memoryPercent
	return
}

func HandleStats(cli *client.Client) {
	for {
		<-time.After(time.Duration(arg.StatsFrequency) * time.Second)
		resp, _ := cli.Info(context.Background())

		var id = resp.Swarm.NodeID
		var memory = MemoryUsage()
		var disk = DiskUsage()
		var cpu = CpuUsage(resp.NCPU)
		var tasks = ContainersUsage(cli)

		status := Status{Id: id, Disk: disk, Cpu: cpu, Memory: memory, Tasks: tasks}
		djuno.SendEvent(djuno.STATS, status)
	}
}

// Container statistic parsers.
// See https://github.com/docker/docker-ce/blob/master/components/cli/cli/command/container/stats_helpers.go

func calculateCPUPercentUnix(previousCPU, previousSystem uint64, v *types.StatsJSON) float64 {
	var (
		cpuPercent  = 0.0
		cpuDelta    = float64(v.CPUStats.CPUUsage.TotalUsage) - float64(previousCPU)
		systemDelta = float64(v.CPUStats.SystemUsage) - float64(previousSystem)
		onlineCPUs  = float64(v.CPUStats.OnlineCPUs)
	)

	if onlineCPUs == 0.0 {
		onlineCPUs = float64(len(v.CPUStats.CPUUsage.PercpuUsage))
	}

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}
	return cpuPercent
}

func calculateCPUPercentWindows(v *types.StatsJSON) float64 {
	possIntervals := uint64(v.Read.Sub(v.PreRead).Nanoseconds())
	possIntervals /= 100
	possIntervals *= uint64(v.NumProcs)
	intervalsUsed := v.CPUStats.CPUUsage.TotalUsage - v.PreCPUStats.CPUUsage.TotalUsage

	if possIntervals > 0 {
		return float64(intervalsUsed) / float64(possIntervals) * 100.0
	}
	return 0.00
}

func calculateMemoryUsageUnixNoCache(mem types.MemoryStats) float64 {
	return float64(mem.Usage - mem.Stats["cache"])
}

func calculateMemoryPercentUnixNoCache(limit float64, usedNoCache float64) float64 {
	if limit != 0 {
		return usedNoCache / limit * 100.0
	}
	return 0
}
