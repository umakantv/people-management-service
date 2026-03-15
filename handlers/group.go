package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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

// AddSubgroup handles POST /groups/{id}/subgroups
// Adds a subgroup to a parent group. Requires AllowSubGroups=true on parent and admin permissions.
func (h *GroupHandler) AddSubgroup(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	requestorID, err := parseRequestorID(r)
	if err != nil {
		writeRequestorError(w, err)
		return
	}

	parentGroupID, err := parseGroupID(r)
	if err != nil {
		writeIDError(w, err)
		return
	}

	var req struct {
		SubgroupID int `json:"subgroup_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body"})
		return
	}
	if req.SubgroupID == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "subgroup_id is required"})
		return
	}

	parentGroup, err := h.groupRepo.GetByID(parentGroupID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "group not found"})
		return
	}

	// Check AllowSubGroups flag
	if parentGroup.AllowSubGroups != 1 {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "subgroups not allowed for this group"})
		return
	}

	// Check admin permissions
	if parentGroup.AdminGroupID == nil {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "admin group not configured"})
		return
	}

	isAdmin, err := h.groupRepo.IsPersonInAdminGroup(*parentGroup.AdminGroupID, requestorID)
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

	// Validate subgroup exists
	subgroup, err := h.groupRepo.GetByID(req.SubgroupID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "subgroup not found"})
		return
	}

	// Prevent self-reference
	if req.SubgroupID == parentGroupID {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "cannot add group as subgroup of itself"})
		return
	}

	// Prevent circular reference - check if parentGroupID is already a subgroup (transitively) of subgroupID
	// If so, adding subgroupID to parentGroupID would create a cycle
	if h.wouldCreateCycle(req.SubgroupID, parentGroupID) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "cannot add subgroup: would create circular reference"})
		return
	}

	if err := h.groupRepo.AddSubgroup(parentGroupID, req.SubgroupID); err != nil {
		logger.Error("Failed to add subgroup", logger.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to add subgroup"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "added",
		"parent_id":  parentGroupID,
		"subgroup_id": subgroup.ID,
	})
}

// wouldCreateCycle checks if adding targetGroupID as subgroup of fromGroupID would create a cycle.
// Specifically, it returns true if fromGroupID is already a transitive subgroup of targetGroupID.
func (h *GroupHandler) wouldCreateCycle(targetGroupID, fromGroupID int) bool {
	// If fromGroupID already has targetGroupID as a (transitive) subgroup, adding target as subgroup of from creates a cycle
	// We check if fromGroupID is reachable from targetGroupID via subgroups
	seen := make(map[int]bool)
	return h.isSubgroupOfRecursive(targetGroupID, fromGroupID, seen)
}

func (h *GroupHandler) isSubgroupOfRecursive(currentGroupID, targetGroupID int, seen map[int]bool) bool {
	if seen[currentGroupID] {
		return false
	}
	seen[currentGroupID] = true

	subgroups, err := h.groupRepo.GetDirectSubgroups(currentGroupID)
	if err != nil {
		return false
	}

	for _, sgID := range subgroups {
		if sgID == targetGroupID {
			return true
		}
		if h.isSubgroupOfRecursive(sgID, targetGroupID, seen) {
			return true
		}
	}
	return false
}

// RemoveSubgroup handles DELETE /groups/{id}/subgroups/{subgroupId}
// Removes a subgroup relationship. Requires admin permissions on parent group.
func (h *GroupHandler) RemoveSubgroup(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	requestorID, err := parseRequestorID(r)
	if err != nil {
		writeRequestorError(w, err)
		return
	}

	parentGroupID, err := parseGroupID(r)
	if err != nil {
		writeIDError(w, err)
		return
	}

	vars := mux.Vars(r)
	subgroupIDStr := vars["subgroupId"]
	subgroupID, err := strconv.Atoi(subgroupIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid subgroup id"})
		return
	}

	parentGroup, err := h.groupRepo.GetByID(parentGroupID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "group not found"})
		return
	}

	// Check admin permissions
	if parentGroup.AdminGroupID == nil {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "admin group not configured"})
		return
	}

	isAdmin, err := h.groupRepo.IsPersonInAdminGroup(*parentGroup.AdminGroupID, requestorID)
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

	// Check subgroup relationship exists (optional - just proceed with delete)
	if err := h.groupRepo.RemoveSubgroup(parentGroupID, subgroupID); err != nil {
		logger.Error("Failed to remove subgroup", logger.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to remove subgroup"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}

// RemoveMember handles DELETE /groups/{id}/members/{personId}
// Removes a person from the group. Only admins can remove members.
// Restrictions:
// - Cannot remove yourself
// - Cannot remove another admin (even if you are an admin)
func (h *GroupHandler) RemoveMember(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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

	vars := mux.Vars(r)
	personIDStr := vars["personId"]
	personID, err := strconv.Atoi(personIDStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid person id"})
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

	// Check requestor is admin
	isRequestorAdmin, err := h.groupRepo.IsPersonInAdminGroup(*group.AdminGroupID, requestorID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to verify admin membership"})
		return
	}
	if !isRequestorAdmin {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "requestor is not admin"})
		return
	}

	// Check: cannot remove yourself
	if personID == requestorID {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "cannot remove yourself from the group"})
		return
	}

	// Check: cannot remove another admin
	isTargetAdmin, err := h.groupRepo.IsPersonInAdminGroup(*group.AdminGroupID, personID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to verify target admin membership"})
		return
	}
	if isTargetAdmin {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "cannot remove another admin from the group"})
		return
	}

	// Validate target person exists (optional - just proceed if not, or could be a direct member check)
	if _, err := h.personRepo.GetByID(personID); err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "person not found"})
		return
	}

	if err := h.groupRepo.RemovePersonFromGroup(personID, groupID, requestorID); err != nil {
		logger.Error("Failed to remove person from group", logger.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to remove person from group"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
}

// BulkMembers handles POST /groups/{id}/bulk-members
// Bulk add or remove multiple members from a group atomically
// Request body: {"person_ids": [1, 2, 3], "action": "add"} or {"action": "remove"}
func (h *GroupHandler) BulkMembers(ctx context.Context, w http.ResponseWriter, r *http.Request) {
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

	var req repository.BulkMembersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON body"})
		return
	}

	// Validate request
	if len(req.PersonIDs) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "person_ids array is required and cannot be empty"})
		return
	}

	// Limit batch size to prevent abuse
	if len(req.PersonIDs) > 100 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "maximum 100 person_ids allowed per request"})
		return
	}

	if req.Action != "add" && req.Action != "remove" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "action must be 'add' or 'remove'"})
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

	// Check admin permissions
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

	// Validate all person IDs exist before proceeding
	for _, personID := range req.PersonIDs {
		if _, err := h.personRepo.GetByID(personID); err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("person not found: %d", personID)})
			return
		}
	}

	var response *repository.BulkMembersResponse

	if req.Action == "add" {
		response, err = h.groupRepo.BulkAddMembersToGroup(req.PersonIDs, groupID, requestorID)
	} else {
		response, err = h.groupRepo.BulkRemoveMembersFromGroup(req.PersonIDs, groupID, requestorID)
	}

	if err != nil {
		logger.Error("Failed to perform bulk operation", logger.Any("error", err), logger.Any("action", req.Action))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to perform bulk operation"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
