package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/umakantv/go-utils/logger"
	"github.com/umakantv/people/models"
	"github.com/umakantv/people/repository"
)

// GroupHandler handles HTTP requests for groups
type GroupHandler struct {
	groupRepo  *repository.GroupRepository
	personRepo *repository.PersonRepository
}

// NewGroupHandler creates a new GroupHandler
func NewGroupHandler(groupRepo *repository.GroupRepository, personRepo *repository.PersonRepository) *GroupHandler {
	return &GroupHandler{groupRepo: groupRepo, personRepo: personRepo}
}

// Create handles POST /groups
// Request includes requestor_id query param
func (h *GroupHandler) Create(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	requestorID, err := parseRequestorID(r)
	if err != nil {
		writeRequestorError(w, err)
		return
	}

	// Validate requestor exists
	if _, err := h.personRepo.GetByID(requestorID); err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "requestor not found"})
		return
	}

	var req models.CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body"})
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "name is required"})
		return
	}

	// Create group
	group, err := h.groupRepo.Create(req)
	if err != nil {
		logger.Error("Failed to create group", logger.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create group"})
		return
	}

	adminGroupID := group.AdminGroupID

	// If admin group not provided, create it automatically
	if adminGroupID == nil {
		adminName := group.Name + "-Admins"
		adminGroup, err := h.groupRepo.Create(models.CreateGroupRequest{
			Name:           adminName,
			Description:    "Admin group for " + group.Name,
			MembersVisible: false,
			AllowSelfAdd:   false,
			AllowSubGroups: false,
			AdminGroupID:   nil,
		})
		if err != nil {
			logger.Error("Failed to create admin group", logger.Any("error", err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to create admin group"})
			return
		}

		adminGroupID = &adminGroup.ID

		// Update group to reference admin group
		_, err = h.groupRepo.Update(group.ID, models.UpdateGroupRequest{AdminGroupID: adminGroupID})
		if err != nil {
			logger.Error("Failed to update group admin_group_id", logger.Any("error", err))
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "failed to set admin group"})
			return
		}
	}

	// Add requestor to admin group and group as direct member
	if err := h.groupRepo.AddPersonToGroup(requestorID, *adminGroupID, requestorID); err != nil {
		logger.Error("Failed to add requestor to admin group", logger.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to add requestor to admin group"})
		return
	}

	if err := h.groupRepo.AddPersonToGroup(requestorID, group.ID, requestorID); err != nil {
		logger.Error("Failed to add requestor to group", logger.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to add requestor to group"})
		return
	}

	// Return group with updated admin_group_id
	finalGroup, err := h.groupRepo.GetByID(group.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to fetch created group"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(finalGroup)
}

// Update handles PUT /groups/{id}
// Requires requestor_id query param and admin access
func (h *GroupHandler) Update(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	requestorID, err := parseRequestorID(r)
	if err != nil {
		writeRequestorError(w, err)
		return
	}

	groupID, err := parseGroupID(r)
	if err != nil {
		writeIDError(w, err)
		return
	}

	group, err := h.groupRepo.GetByID(groupID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "group not found"})
		return
	}

	if group.AdminGroupID == nil {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "admin group not configured"})
		return
	}

	isAdmin, err := h.groupRepo.IsPersonInAdminGroup(*group.AdminGroupID, requestorID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to verify admin membership"})
		return
	}
	if !isAdmin {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "requestor is not admin"})
		return
	}

	var req models.UpdateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body"})
		return
	}

	updated, err := h.groupRepo.Update(groupID, req)
	if err != nil {
		logger.Error("Failed to update group", logger.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to update group"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updated)
}

// AddMember handles POST /groups/{id}/members
// Requires requestor_id query param
func (h *GroupHandler) AddMember(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	requestorID, err := parseRequestorID(r)
	if err != nil {
		writeRequestorError(w, err)
		return
	}

	groupID, err := parseGroupID(r)
	if err != nil {
		writeIDError(w, err)
		return
	}

	var req struct {
		PersonID int `json:"person_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body"})
		return
	}
	if req.PersonID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "person_id is required"})
		return
	}

	group, err := h.groupRepo.GetByID(groupID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "group not found"})
		return
	}

	if group.AdminGroupID == nil {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "admin group not configured"})
		return
	}

	isAdmin, err := h.groupRepo.IsPersonInAdminGroup(*group.AdminGroupID, requestorID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to verify admin membership"})
		return
	}
	if !isAdmin {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "requestor is not admin"})
		return
	}

	// Validate target person exists
	if _, err := h.personRepo.GetByID(req.PersonID); err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "person not found"})
		return
	}

	if err := h.groupRepo.AddPersonToGroup(req.PersonID, groupID, requestorID); err != nil {
		logger.Error("Failed to add person to group", logger.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to add person to group"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "added"})
}

// GetDirectMembers handles GET /groups/{id}/members/direct
// Returns direct people and subgroups
func (h *GroupHandler) GetDirectMembers(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	groupID, err := parseGroupID(r)
	if err != nil {
		writeIDError(w, err)
		return
	}

	// Ensure group exists
	if _, err := h.groupRepo.GetByID(groupID); err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "group not found"})
		return
	}

	people, subgroups, err := h.groupRepo.GetDirectPeopleAndSubgroups(groupID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to fetch direct members"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"people":   people,
		"subgroups": subgroups,
	})
}

// IsMember handles GET /groups/{id}/members/{personId}/check
func (h *GroupHandler) IsMember(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	groupID, err := parseGroupID(r)
	if err != nil {
		writeIDError(w, err)
		return
	}

	vars := mux.Vars(r)
	personIDStr := vars["personId"]
	personID, err := strconv.Atoi(personIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid person id"})
		return
	}

	// Ensure group exists
	if _, err := h.groupRepo.GetByID(groupID); err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "group not found"})
		return
	}

	isMember, err := h.groupRepo.IsPersonInGroup(personID, groupID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to check membership"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"is_member": isMember})
}

func parseRequestorID(r *http.Request) (int, error) {
	requestorIDStr := r.Header.Get("X-Person-Id")
	if requestorIDStr == "" {
		return 0, errBadRequest("X-Person-Id header is required")
	}
	requestorID, err := strconv.Atoi(requestorIDStr)
	if err != nil {
		return 0, errBadRequest("invalid X-Person-Id header")
	}
	return requestorID, nil
}

func parseGroupID(r *http.Request) (int, error) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	if idStr == "" {
		return 0, errBadRequest("group id is required")
	}
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, errBadRequest("invalid group id")
	}
	return id, nil
}

type requestError struct {
	status int
	msg    string
}

func (e requestError) Error() string {
	return e.msg
}

func errBadRequest(msg string) requestError {
	return requestError{status: http.StatusBadRequest, msg: msg}
}

func writeRequestorError(w http.ResponseWriter, err error) {
	if re, ok := err.(requestError); ok {
		w.WriteHeader(re.status)
		json.NewEncoder(w).Encode(map[string]string{"error": re.msg})
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": "invalid X-Person-Id header"})
}

func writeIDError(w http.ResponseWriter, err error) {
	if re, ok := err.(requestError); ok {
		w.WriteHeader(re.status)
		json.NewEncoder(w).Encode(map[string]string{"error": re.msg})
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": "invalid id"})
}
