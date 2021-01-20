# Djuno agent
![Image of Djuno Logo](https://djuno.io/wp-content/uploads/2020/06/cropped-Djuno-just-logo-1-transparent.png)


Djuno pass monitoring docker agent 


[![Build Status](https://travis-ci.com/Djuno-Ltd/agent.svg?token=qqHq1v8srFQ4DXwKgnW2&branch=master)](https://travis-ci.com/Djuno-Ltd/agent)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://github.com/Djuno-Ltd/agent/pulls)
[![MIT Licensed](https://img.shields.io/badge/license-MIT-brightgreen.svg?style=flat-square)](LICENSE.md)
[![Docker Pulls](https://img.shields.io/docker/pulls/djunoltd/agent.svg?style=flat-square)](https://hub.docker.com/repository/docker/djunoltd/agent/general)
## Run

```{r, engine='bash', count_lines}
docker run -d \
  --name agent \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  djunoltd/agent:latest
```

### Parameters

- STATS_FREQUENCY - default to **30**
- EVENT_ENDPOINT - default to **http://app:8080/events**
- HEALTH_CHECK_ENDPOINT - default to **http://app:8080/version**
- DEBUG_EVENT - default to **false**
- DEBUG_STATS - default to **false**

