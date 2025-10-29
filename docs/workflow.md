# High-Level Open Scheduler Workflow

```mermaid
sequenceDiagram
  participant Client
  participant ControlPlane
  participant Scheduler
  participant DB
  participant NodeAgent
  participant LocalQueue
  participant ProviderDriver
  participant Provider

  Client->>ControlPlane: SubmitJob(jobSpec)
  ControlPlane->>DB: INSERT job status=Pending
  ControlPlane->>Scheduler: notify pending job
  Scheduler->>DB: SELECT nodes (snapshot)
  Scheduler->>DB: BEGIN TX and reserve node resources
  Scheduler->>DB: UPDATE job.assigned_node and set job.status = Reserved
  Scheduler->>DB: CREATE binding with status = Reserved
  Scheduler->>NodeAgent: BindCommand(job_id, spec, idempotency_token)
  NodeAgent->>LocalQueue: enqueue BindCommand
  LocalQueue->>ProviderDriver: dequeue -> CreateInstance(spec, client_token)
  ProviderDriver->>Provider: POST /1.0/instances  (returns operation_id)
  ProviderDriver->>LocalQueue: persist mapping op_id <-> job_id
  LocalQueue->>ProviderDriver: poll /1.0/operations/op_id until done
  Provider->>ProviderDriver: operation done (success + instance_name)
  LocalQueue->>NodeAgent: InstanceStatus(job_id, instance_name, state=Created)
  NodeAgent->>ControlPlane: InstanceStatus (via NodeStream)
  ControlPlane->>DB: UPDATE binding status = Created and set instance id
  ControlPlane->>DB: UPDATE job.status = Running
  ControlPlane->>Client: JobStatus(Running, node=...)
```