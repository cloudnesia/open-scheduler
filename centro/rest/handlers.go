package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	etcdstorage "github.com/open-scheduler/centro/storage/etcd"
	pb "github.com/open-scheduler/proto"
	httpSwagger "github.com/swaggo/http-swagger"
)

type APIServer struct {
	storage *etcdstorage.Storage
	router  *mux.Router
}

func NewAPIServer(storage *etcdstorage.Storage) *APIServer {
	server := &APIServer{
		storage: storage,
		router:  mux.NewRouter(),
	}

	server.setupRoutes()
	return server
}

func (s *APIServer) setupRoutes() {
	// Swagger documentation
	s.router.PathPrefix("/swagger/").Handler(httpSwagger.WrapHandler)

	api := s.router.PathPrefix("/api/v1").Subrouter()

	api.HandleFunc("/auth/login", s.handleLogin).Methods("POST", "OPTIONS")

	protected := api.PathPrefix("").Subrouter()
	protected.Use(JWTAuthMiddleware)

	protected.HandleFunc("/jobs", s.handleListJobs).Methods("GET")
	protected.HandleFunc("/jobs", s.handleSubmitJob).Methods("POST")	
	protected.HandleFunc("/jobs/{id}", s.handleGetJob).Methods("GET")
	protected.HandleFunc("/jobs/{id}/status", s.handleGetJobStatus).Methods("GET")
	protected.HandleFunc("/jobs/{id}/events", s.handleGetJobEvents).Methods("GET")
	protected.HandleFunc("/instances", s.handleListInstances).Methods("GET")
	protected.HandleFunc("/instances/{id}", s.handleGetInstanceData).Methods("GET")

	protected.HandleFunc("/nodes", s.handleListNodes).Methods("GET")
	protected.HandleFunc("/nodes/{id}", s.handleGetNode).Methods("GET")
	protected.HandleFunc("/nodes/{id}/health", s.handleNodeHealth).Methods("GET")

	protected.HandleFunc("/stats", s.handleStats).Methods("GET")

	s.router.Use(LoggingMiddleware)
	s.router.Use(CORSMiddleware)
}

func (s *APIServer) GetRouter() *mux.Router {
	return s.router
}

type LoginRequest struct {
	Username string `json:"username" example:"admin"`
	Password string `json:"password" example:"admin123"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	Username  string `json:"username"`
}

// handleLogin godoc
// @Summary Login to get JWT token
// @Description Authenticate with username and password to receive a JWT token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param login body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /auth/login [post]
func (s *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		respondWithError(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	// Verify credentials using user manager with bcrypt
	um := GetUserManager()
	user, err := um.VerifyCredentials(req.Username, req.Password)
	if err != nil {
		log.Printf("[Centro REST] Login failed for user %s: %v", req.Username, err)
		respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Generate JWT token
	token, err := GenerateToken(user.Username, user.Role)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	respondWithJSON(w, http.StatusOK, LoginResponse{
		Token:     token,
		ExpiresIn: 86400,
		Username:  user.Username,
	})
}

// handleListJobs godoc
// @Summary List all jobs
// @Description Get a list of all jobs with optional status filter
// @Tags Jobs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param status query string false "Filter by status (queued, pending, completed, failed)"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /jobs [get]
func (s *APIServer) handleListJobs(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	statusFilter := r.URL.Query().Get("status")

	response := make(map[string]interface{})

	// Queued jobs - waiting in queue to be picked up
	if statusFilter == "" || statusFilter == "queued" {
		queueLength, err := s.storage.GetQueueLength(ctx)
		if err != nil {
			log.Printf("[Centro REST] Failed to get queue length: %v", err)
		} else {
			response["queued_count"] = queueLength
		}
	}

	if statusFilter == "" || statusFilter == "queued" {
		queueJobs, err := s.storage.GetQueueJobs(ctx)
		if err != nil {
			log.Printf("[Centro REST] Failed to get queue jobs: %v", err)
		} else {
			response["queued_jobs"] = queueJobs
		}
	}

	// Pending jobs - claimed by agent but not completed yet (assigned/running)
	if statusFilter == "" || statusFilter == "active" {
		activeJobs, err := s.storage.GetAllActiveJobs(ctx)
		if err != nil {
			log.Printf("[Centro REST] Failed to get active jobs: %v", err)
		} else {
			jobs := make([]map[string]interface{}, 0, len(activeJobs))
			for jobID, status := range activeJobs {
				jobs = append(jobs, map[string]interface{}{
					"job_id":     jobID,
					"node_id":    status.NodeID,
					"status":     status.Status,
					"detail":     status.Detail,
					"updated_at": status.UpdatedAt,
					"claimed_at": status.ClaimedAt,
					"job":        status.Job,
				})
			}
			response["active_jobs"] = jobs
		}
	}

	// Get all history to filter by status
	allHistory, err := s.storage.GetAllJobHistory(ctx)
	if err != nil {
		log.Printf("[Centro REST] Failed to get job history: %v", err)
	} else {
		// Completed jobs - successfully finished
		if statusFilter == "" || statusFilter == "completed" {
			completedJobs := make([]map[string]interface{}, 0)
			for jobID, status := range allHistory {
				if status.Status == "completed" {
					completedJobs = append(completedJobs, map[string]interface{}{
						"job_id":     jobID,
						"node_id":    status.NodeID,
						"status":     status.Status,
						"detail":     status.Detail,
						"updated_at": status.UpdatedAt,
						"claimed_at": status.ClaimedAt,
						"job":        status.Job,
					})
				}
			}
			response["completed_jobs"] = completedJobs
			response["completed_count"] = len(completedJobs)
		}

		// Failed jobs - permanently failed (exceeded retries)
		if statusFilter == "" || statusFilter == "failed" {
			failedJobs := make([]map[string]interface{}, 0)
			for jobID, status := range allHistory {
				if status.Status == "failed" {
					failedJobs = append(failedJobs, map[string]interface{}{
						"job_id":     jobID,
						"node_id":    status.NodeID,
						"status":     "failed_permanent",
						"detail":     status.Detail,
						"updated_at": status.UpdatedAt,
						"claimed_at": status.ClaimedAt,
						"job":        status.Job,
					})
				}
			}

			// Also include jobs in failed queue (pending retry)
			failedQueueJobs, err := s.storage.GetAllFailedJobs(ctx)
			if err != nil {
				log.Printf("[Centro REST] Failed to get failed queue jobs: %v", err)
			} else {
				for _, job := range failedQueueJobs {
					failedJobs = append(failedJobs, map[string]interface{}{
						"job_id":     job.JobId,
						"node_id":    "",
						"status":     "failed_retrying",
						"detail":     fmt.Sprintf("Job failed, pending retry (attempt %d/%d)", job.RetryCount, job.MaxRetries),
						"updated_at": nil,
						"claimed_at": nil,
						"job":        job,
					})
				}
			}

			response["failed_jobs"] = failedJobs
			response["failed_count"] = len(failedJobs)
		}
	}

	respondWithJSON(w, http.StatusOK, response)
}

type SubmitJobRequest struct {
	JobId            string               `json:"job_id" example:"123"`
	JobName          string               `json:"job_name" example:"web-server-job"`
	JobType          string               `json:"job_type" example:"service"`
	SelectedClusters []string             `json:"selected_clusters" example:"dc1,dc2"`
	Meta             map[string]string    `json:"meta"`
	Driver           string               `json:"driver" example:"podman"`
	WorkloadType     string               `json:"workload_type" example:"container"`
	Command          string               `json:"command" example:"echo 'Hello World'"`
	InstanceConfig   *InstanceSpecRequest `json:"instance_config,omitempty"`
	Resources        *ResourcesRequest    `json:"resources,omitempty"`
	Volumes          []VolumeRequest      `json:"volumes,omitempty"`
}

type ResourcesRequest struct {
	MemoryMB        int64   `json:"memory_mb" example:"512"`
	MemoryReserveMB int64   `json:"memory_reserve_mb,omitempty" example:"256"`
	CPU             float32 `json:"cpu" example:"1.0"`
	CPUReserve      float32 `json:"cpu_reserve,omitempty" example:"0.5"`
}

type VolumeRequest struct {
	HostPath     string `json:"host_path" example:"/data/app"`
	InstancePath string `json:"instance_path" example:"/usr/share/nginx/html"`
	ReadOnly     bool   `json:"read_only,omitempty" example:"false"`
}

type InstanceSpecRequest struct {
	Image   string            `json:"image" example:"docker.io/library/alpine:latest"`
	Command []string          `json:"command,omitempty" example:""`
	Args    []string          `json:"args,omitempty" example:""`
	Options map[string]string `json:"options,omitempty"`
}

// handleSubmitJob godoc
// @Summary Submit a new job
// @Description Create and submit a new job to the scheduler
// @Tags Jobs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param job body SubmitJobRequest true "Job details"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /jobs [post]
func (s *APIServer) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	var req SubmitJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Comprehensive input validation
	if err := ValidateJobRequest(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Validation error: %v", err))
		return
	}
	jobID := uuid.New().String()

	job := &pb.Job{
		JobId:            jobID,
		JobName:          req.JobName,
		JobType:          req.JobType,
		SelectedClusters: req.SelectedClusters,
		DriverType:       req.Driver,
		WorkloadType:     req.WorkloadType,
		Command:          req.Command,
		RetryCount:       0,
		MaxRetries:       3, // Default: retry up to 3 times
		LastRetryTime:    0,
	}
	if req.InstanceConfig != nil {
		job.InstanceConfig = &pb.InstanceSpec{
			ImageName:     req.InstanceConfig.Image,
			Entrypoint:    req.InstanceConfig.Command,
			Arguments:     req.InstanceConfig.Args,
			DriverOptions: req.InstanceConfig.Options,
		}
	}
	if req.Resources != nil {
		job.ResourceRequirements = &pb.Resources{
			MemoryLimitMb:    req.Resources.MemoryMB,
			MemoryReservedMb: req.Resources.MemoryReserveMB,
			CpuLimitCores:    req.Resources.CPU,
			CpuReservedCores: req.Resources.CPUReserve,
		}
	}
	if len(req.Volumes) > 0 {
		job.VolumeMounts = make([]*pb.Volume, 0, len(req.Volumes))
		for _, v := range req.Volumes {
			job.VolumeMounts = append(job.VolumeMounts, &pb.Volume{
				SourcePath: v.HostPath,
				TargetPath: v.InstancePath,
				ReadOnly:   v.ReadOnly,
			})
		}
	}
	if req.Meta != nil {
		job.JobMetadata = req.Meta
	}

	ctx := context.Background()
	if err := s.storage.EnqueueJob(ctx, job); err != nil {
		log.Printf("[Centro REST] Failed to enqueue job: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to submit job")
		return
	}

	log.Printf("[Centro REST] Job submitted: %s (%s)", jobID, req.JobName)

	respondWithJSON(w, http.StatusCreated, map[string]interface{}{
		"job_id":  jobID,
		"message": "Job submitted successfully",
		"job":     job,
	})
}

// handleGetJob godoc
// @Summary Get job details
// @Description Get detailed information about a specific job
// @Tags Jobs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /jobs/{id} [get]
func (s *APIServer) handleGetJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	if jobID == "" {
		respondWithError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	ctx := context.Background()

	activeJob, err := s.storage.GetJobActive(ctx, jobID)
	if err != nil {
		log.Printf("[Centro REST] Failed to get active job: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve job")
		return
	}

	events, err := s.storage.GetJobEvents(ctx, jobID)
	if err != nil {
		log.Printf("[Centro REST] Failed to get job events: %v", err)
		// Events are secondary, continue without them
		events = nil
	}

	if activeJob != nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"job_id":     jobID,
			"status":     "active",
			"node_id":    activeJob.NodeID,
			"detail":     activeJob.Detail,
			"updated_at": activeJob.UpdatedAt,
			"claimed_at": activeJob.ClaimedAt,
			"job":        activeJob.Job,
			"events":     events,
		})
		return
	}
	
	queueJob, err := s.storage.GetQueueJob(ctx, jobID)
	if err != nil && err.Error() != "job not found" {
		log.Printf("[Centro REST] Failed to get queue job: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve job")
		return
	}

	if queueJob != nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"job_id":     jobID,
			"status":     "queued",
			"node_id":    "",
			"detail":     fmt.Sprintf("Job queued (attempt %d/%d)", queueJob.RetryCount, queueJob.MaxRetries),
			"updated_at": nil,
			"claimed_at": nil,
			"job":        queueJob,
			"events":     events,
		})
		return
	}

	failedJob, err := s.storage.GetFailedJob(ctx, jobID)
	if err != nil && err.Error() != "job not found" {
		log.Printf("[Centro REST] Failed to get failed job: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve job")
		return
	}

	if failedJob != nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"job_id":     jobID,
			"status":     "failed",
			"node_id":    "",
			"detail":     fmt.Sprintf("Job failed, pending retry (attempt %d/%d)", failedJob.RetryCount, failedJob.MaxRetries),
			"updated_at": nil,
			"claimed_at": nil,
			"job":        failedJob,
			"events":     events,
		})
		return
	}

	historyJob, err := s.storage.GetJobHistory(ctx, jobID)
	if err != nil {
		log.Printf("[Centro REST] Failed to get job history: %v", err)
	}

	if historyJob != nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"job_id":     jobID,
			"status":     "completed",
			"node_id":    historyJob.NodeID,
			"detail":     historyJob.Detail,
			"updated_at": historyJob.UpdatedAt,
			"claimed_at": historyJob.ClaimedAt,
			"job":        historyJob.Job,
			"events":     events,
		})
		return
	}

	respondWithError(w, http.StatusNotFound, "Job not found")
}

// handleGetJobStatus godoc
// @Summary Get job status
// @Description Get the current status of a specific job
// @Tags Jobs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /jobs/{id}/status [get]
func (s *APIServer) handleGetJobStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	if jobID == "" {
		respondWithError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	ctx := context.Background()

	activeJob, err := s.storage.GetJobActive(ctx, jobID)
	if err != nil && err.Error() != "job not found" {
		log.Printf("[Centro REST] Failed to get active job: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve job status")
		return
	}

	if activeJob != nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"job_id":     jobID,
			"status":     activeJob.Status,
			"node_id":    activeJob.NodeID,
			"detail":     activeJob.Detail,
			"updated_at": activeJob.UpdatedAt,
		})
		return
	}

	historyJob, err := s.storage.GetJobHistory(ctx, jobID)
	if err != nil && err.Error() != "job not found" {
		log.Printf("[Centro REST] Failed to get job history: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve job status")
		return
	}

	if historyJob != nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"job_id":     jobID,
			"status":     historyJob.Status,
			"node_id":    historyJob.NodeID,
			"detail":     historyJob.Detail,
			"updated_at": historyJob.UpdatedAt,
		})
		return
	}

	respondWithError(w, http.StatusNotFound, "Job not found")
}

// handleListInstances godoc
// @Summary List all instances
// @Description Get a list of all instances in the cluster
// @Tags Instances
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /instances [get]
func (s *APIServer) handleListInstances(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	instances, err := s.storage.GetListOfInstances(ctx)
	if err != nil {
		log.Printf("[Centro REST] Failed to get instances: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve instances")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"instances": instances,
		"count":     len(instances),
	})
}

// handleGetJobEvents godoc
// @Summary Get job events
// @Description Retrieve events for a specific job
// @Tags Jobs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /jobs/{id}/events [get]
func (s *APIServer) handleGetJobEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	ctx := context.Background()
	events, err := s.storage.GetJobEvents(ctx, jobID)
	if err != nil {
		log.Printf("[Centro REST] Failed to get job events: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve events")
		return
	}

	if events == nil {
		respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"job_id": jobID,
			"events": []string{},
		})
		return
	}

	// Filter out consecutive duplicate events
	filteredEvents := filterConsecutiveDuplicates(events)

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"job_id": jobID,
		"events": filteredEvents,
	})
}

// filterConsecutiveDuplicates removes consecutive duplicate events from the list
func filterConsecutiveDuplicates(events []string) []string {
	if len(events) == 0 {
		return events
	}

	filtered := make([]string, 0, len(events))
	filtered = append(filtered, events[0])

	for i := 1; i < len(events); i++ {
		// Extract the message part after the timestamp (everything after "] ")
		currentMsg := extractEventMessage(events[i])
		previousMsg := extractEventMessage(events[i-1])

		// Only add if the message is different from the previous one
		if currentMsg != previousMsg {
			filtered = append(filtered, events[i])
		}
	}

	return filtered
}

// extractEventMessage extracts the message part from an event string
// Events format: "[timestamp] message"
func extractEventMessage(event string) string {
	// Find the position after the timestamp bracket
	idx := strings.Index(event, "] ")
	if idx == -1 {
		return event
	}
	return event[idx+2:]
}

// handleGetInstanceData godoc
// @Summary Get instance data for a job
// @Description Retrieves the instance data associated with a specific job
// @Tags Instances
// @Security BearerAuth
// @Produce json
// @Param id path string true "Instance ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /instances/{id} [get]
func (s *APIServer) handleGetInstanceData(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID := vars["id"]

	ctx := context.Background()
	instanceData, err := s.storage.GetInstanceData(ctx, jobID)
	if err != nil {
		log.Printf("[Centro REST] Failed to get instance data: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve instance data")
		return
	}

	if instanceData == nil {
		respondWithError(w, http.StatusNotFound, "Instance data not found for this job")
		return
	}

	events, err := s.storage.GetJobEvents(ctx, jobID)
	if err != nil {
		log.Printf("[Centro REST] Failed to get job events: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve events")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"job_id":        jobID,
		"instance_data": instanceData,
		"events":        events,
	})
}

// handleListNodes godoc
// @Summary List all nodes
// @Description Get a list of all registered nodes in the cluster
// @Tags Nodes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /nodes [get]
func (s *APIServer) handleListNodes(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	nodes, err := s.storage.GetAllNodes(ctx)
	if err != nil {
		log.Printf("[Centro REST] Failed to get nodes: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve nodes")
		return
	}

	nodesList := make([]map[string]interface{}, 0, len(nodes))
	for _, node := range nodes {
		nodesList = append(nodesList, map[string]interface{}{
			"node_id":        node.NodeID,
			"last_heartbeat": node.LastHeartbeat,
			"ram_mb":         node.RamMB,
			"cpu_cores":      node.CPUCores,
			"disk_mb":        node.DiskMB,
			"metadata":       node.Metadata,
		})
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"nodes": nodesList,
		"count": len(nodesList),
	})
}

// handleGetNode godoc
// @Summary Get node details
// @Description Get detailed information about a specific node
// @Tags Nodes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /nodes/{id} [get]
func (s *APIServer) handleGetNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["id"]

	ctx := context.Background()
	node, err := s.storage.GetNode(ctx, nodeID)
	if err != nil {
		log.Printf("[Centro REST] Failed to get node: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve node")
		return
	}

	if node == nil {
		respondWithError(w, http.StatusNotFound, "Node not found")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":        node.NodeID,
		"last_heartbeat": node.LastHeartbeat,
		"ram_mb":         node.RamMB,
		"cpu_cores":      node.CPUCores,
		"disk_mb":        node.DiskMB,
		"metadata":       node.Metadata,
	})
}

// handleNodeHealth godoc
// @Summary Check node health
// @Description Check the health status of a specific node
// @Tags Nodes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Node ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /nodes/{id}/health [get]
func (s *APIServer) handleNodeHealth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["id"]

	ctx := context.Background()
	node, err := s.storage.GetNode(ctx, nodeID)
	if err != nil {
		log.Printf("[Centro REST] Failed to get node: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve node")
		return
	}

	if node == nil {
		respondWithError(w, http.StatusNotFound, "Node not found")
		return
	}

	healthy := node.IsHealthy()
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":        node.NodeID,
		"healthy":        healthy,
		"status":         status,
		"last_heartbeat": node.LastHeartbeat,
	})
}

// handleStats godoc
// @Summary Get system statistics
// @Description Get overall system statistics including node and job counts
// @Tags Statistics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /stats [get]
func (s *APIServer) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	nodes, err := s.storage.GetAllNodes(ctx)
	if err != nil {
		log.Printf("[Centro REST] Failed to get nodes: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve stats")
		return
	}

	queueLength, err := s.storage.GetQueueLength(ctx)
	if err != nil {
		log.Printf("[Centro REST] Failed to get queue length: %v", err)
		queueLength = 0
	}

	activeCount, err := s.storage.GetActiveJobCount(ctx)
	if err != nil {
		log.Printf("[Centro REST] Failed to get active job count: %v", err)
		activeCount = 0
	}

	completedCount, err := s.storage.GetJobHistoryCount(ctx)
	if err != nil {
		log.Printf("[Centro REST] Failed to get history count: %v", err)
		completedCount = 0
	}

	healthyNodes := 0
	for _, node := range nodes {
		if node.IsHealthy() {
			healthyNodes++
		}
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"nodes": map[string]interface{}{
			"total":   len(nodes),
			"healthy": healthyNodes,
		},
		"jobs": map[string]interface{}{
			"queued":    queueLength,
			"active":    activeCount,
			"completed": completedCount,
		},
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Internal server error"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}
