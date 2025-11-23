# Open Scheduler - Quick Start Guide

Get up and running with Open Scheduler in less than 10 minutes!

## Overview

**Open Scheduler** is a distributed job scheduler that manages workloads across multiple nodes. It consists of:
- **Centro**: Control plane that handles job scheduling and persistence
- **Agents**: Workers that execute jobs using Podman, Incus, or native processes
- **Web Panel**: Modern UI for managing jobs and nodes
- **CLI (osctl)**: Command-line tool for programmatic access

## Prerequisites

Before starting, ensure you have:

- **Go** 1.20+ ([install](https://golang.org/doc/install))
- **etcd** 3.5+ ([install](https://etcd.io/docs/v3.5/install/))
- **Node.js** 18+ ([install](https://nodejs.org/))
- **Podman** 4.0+ or **Docker** ([install](https://podman.io/)) - optional, for container workloads
- **git** for cloning the repository

### Quick Prerequisite Check

```bash
go version        # Should be 1.20+
etcd --version    # Should be v3.5+
node --version    # Should be 18+
podman --version  # Should be 4.0+ (optional)
```

## Step 1: Clone & Setup

```bash
# Clone the repository
git clone https://github.com/your-org/open-scheduler.git
cd open-scheduler

# Download Go dependencies
go mod tidy
```

## Step 2: Start etcd (Persistent Storage)

etcd stores all job, node, and instance data. Start it in a separate terminal:

```bash
# Start etcd on port 2379
etcd --data-dir=default.etcd \
     --listen-client-urls=http://localhost:2379 \
     --advertise-client-urls=http://localhost:2379

# Output should show: "Ready to accept client connections"
```

**Verify etcd is running:**
```bash
etcdctl --endpoints=localhost:2379 get --prefix "/" | head -5
```

## Step 3: Build All Components

In the project root, build all binaries:

```bash
# Build Centro (control plane)
go build -o centro_server ./centro

# Build Agent (worker)
go build -o agent_client ./agent

# Build CLI
go build -o osctl ./cli
```

Verify builds were successful:
```bash
ls -la | grep -E "centro_server|agent_client|osctl"
```

## Step 4: Start Centro (Control Plane)

Start the Centro server in a new terminal:

```bash
./centro_server --etcd-endpoints=localhost:2379

# Output should show:
# 2024/01/15 10:30:00 Starting gRPC server on :50051
# 2024/01/15 10:30:00 Starting REST server on :8080
# 2024/01/15 10:30:01 Scheduler running...
```

**Verify Centro is running:**
```bash
curl http://localhost:8080/api/nodes
```

## Step 5: Start Agent (Worker)

Start one or more agents in separate terminals:

```bash
./agent_client --server=localhost:50051 --token=test-token

# Output should show:
# 2024/01/15 10:30:02 Registering with Centro...
# 2024/01/15 10:30:02 Heartbeat sent successfully
# 2024/01/15 10:30:02 Waiting for jobs...
```

Start multiple agents (in different terminals) to simulate a cluster:
```bash
./agent_client --server=localhost:50051 --token=test-token --node-id=worker-2
./agent_client --server=localhost:50051 --token=test-token --node-id=worker-3
```

## Step 6: Access the Web Panel

In another terminal, start the web UI:

```bash
cd panel
npm install    # First time only
npm run dev

# Output should show:
#   Local: http://localhost:3000/
```

Open your browser to **http://localhost:3000**

**Default credentials:**
- Username: `admin`
- Password: `admin123`

You should see the dashboard with registered nodes and job statistics.

## Step 7: Submit Your First Job

### Option A: Using the Web Panel

1. Go to **Jobs** tab in the panel
2. Click **Create Job**
3. Fill in the form:
   - **Job ID:** `my-first-job`
   - **Job Name:** `My First Job`
   - **Driver Type:** `podman`
   - **Image:** `alpine:latest`
   - **Command:** `echo "Hello from Open Scheduler!"`
4. Click **Submit**
5. Check **Jobs** tab to see the job execute

### Option B: Using CLI (osctl)

First, login:
```bash
./osctl login --username admin --password admin123
```

Then submit a job from the example spec:
```bash
./osctl apply -f cli/sample-job.yaml
```

Check job status:
```bash
./osctl get jobs              # List all jobs
./osctl get jobs --active     # Show only active jobs
./osctl get jobs --failed     # Show only failed jobs
./osctl describe job <JOB_ID> # Show job details
```

### Option C: Using REST API

```bash
# Create a job
curl -X POST http://localhost:8080/api/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "job_id": "rest-test-job",
    "job_name": "REST Test Job",
    "job_type": "single",
    "driver_type": "podman",
    "instance_config": {
      "image_name": "alpine:latest",
      "entrypoint": ["/bin/sh"],
      "arguments": ["-c", "echo Hello REST API!"]
    },
    "max_retries": 2
  }'

# List all jobs
curl http://localhost:8080/api/jobs

# Get job details
curl http://localhost:8080/api/jobs/<JOB_ID>

# List nodes
curl http://localhost:8080/api/nodes
```

**View Swagger/OpenAPI docs:** http://localhost:8080/swagger/index.html

## Step 8: Monitor Your Jobs

### Via Web Panel
- Check **Dashboard** for overall stats
- Go to **Jobs** to see job progress
- Click on a job to view events and logs
- Visit **Nodes** to monitor worker health

### Via CLI
```bash
# Watch active jobs
watch -n 2 './osctl get jobs --active'

# Get detailed job info
./osctl describe job my-first-job

# List all nodes
./osctl get nodes

# Check node details
./osctl describe node <NODE_ID>
```

### Via REST API
```bash
# Get job events
curl http://localhost:8080/api/jobs/<JOB_ID>/events

# Get instance data
curl http://localhost:8080/api/instances/<JOB_ID>

# Get node metrics
curl http://localhost:8080/api/nodes/<NODE_ID>/instances
```

## All-in-One Quick Start Script

If you want to start everything at once, use the provided demo script:

```bash
bash demo.sh
```

This will:
1. Kill any existing processes
2. Start etcd
3. Build all components
4. Start Centro
5. Start one Agent
6. Start the web panel
7. Display log file paths for monitoring

To stop everything:
```bash
pkill etcd
pkill centro_server
pkill agent_client
```

## Common Commands Reference

| Task | Command |
|------|---------|
| **Start etcd** | `etcd --data-dir=default.etcd --listen-client-urls=http://localhost:2379` |
| **Start Centro** | `./centro_server --etcd-endpoints=localhost:2379` |
| **Start Agent** | `./agent_client --server=localhost:50051 --token=test-token` |
| **Start Panel** | `cd panel && npm run dev` |
| **List jobs** | `./osctl get jobs` |
| **Submit job** | `./osctl apply -f job.yaml` |
| **View logs** | `curl http://localhost:8080/api/jobs/<ID>/events` |
| **Check nodes** | `./osctl get nodes` |
| **Access Swagger** | `http://localhost:8080/swagger/index.html` |
| **Access Panel** | `http://localhost:3000` |

## Ports Reference

| Service | Port | Purpose |
|---------|------|---------|
| **etcd** | 2379 | Persistent data storage |
| **Centro gRPC** | 50051 | Agent-Centro communication |
| **Centro REST API** | 8080 | REST API & Swagger UI |
| **Web Panel** | 3000 | Web-based management UI |

## Next Steps

Now that you have the basics running, explore:

1. **Create complex jobs** with multiple containers and volume mounts
2. **Set retry policies** for failed jobs
3. **Use different drivers**: `podman`, `incus` (for VMs), or `process` (native execution)
4. **Monitor job lifecycles** through the web panel and REST API
5. **Write custom CLI scripts** using osctl for automation
6. **Deploy to a real cluster** following [CENTRO.md](README/CENTRO.md) and [AGENT.md](README/AGENT.md)

## Troubleshooting

### etcd Connection Error
```
Error: failed to dial etcd
```
**Solution:** Ensure etcd is running on localhost:2379
```bash
etcdctl --endpoints=localhost:2379 endpoint health
```

### Centro Won't Start
```
Error: port 8080/50051 already in use
```
**Solution:** Kill existing processes
```bash
lsof -i :8080
lsof -i :50051
kill -9 <PID>
```

### Agent Can't Connect to Centro
```
Error: failed to connect to localhost:50051
```
**Solution:**
- Ensure Centro is running: `netstat -an | grep 50051`
- Check network connectivity: `ping localhost`
- Verify server address: `./agent_client --server=localhost:50051`

### Panel Shows "Cannot fetch data"
```
Error: Failed to fetch from /api/jobs
```
**Solution:**
- Ensure Centro is running on port 8080
- Check browser console for CORS errors
- Verify API is accessible: `curl http://localhost:8080/api/jobs`

### Container Jobs Won't Execute
```
Error: No driver available for 'podman'
```
**Solution:**
- Install Podman: `apt-get install podman` or `brew install podman`
- Verify Podman access: `podman version`
- Change job driver to `process` for testing

## Getting Help

- **Documentation Index**: See [README/INDEX.md](README/INDEX.md)
- **Architecture Details**: See [README/SCHEDULER.md](README/SCHEDULER.md)
- **API Documentation**: See [README/REST_API_IMPLEMENTATION.md](README/REST_API_IMPLEMENTATION.md)
- **etcd Setup**: See [README/ETCD_SETUP.md](README/ETCD_SETUP.md)
- **GitHub Issues**: Report bugs on the project repository

## What's Next?

Once comfortable with the basics:

1. **Scale up**: Run agents on multiple machines
2. **Production setup**: Use managed etcd and configure authentication
3. **Job templates**: Create reusable job configurations
4. **Monitoring**: Integrate with Prometheus/Grafana
5. **Advanced features**: Explore job dependencies, cron jobs, and advanced scheduling

Happy scheduling! 🚀
