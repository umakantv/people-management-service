package repository

import (
	"database/sql"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/umakantv/people/models"
)

// GroupRepository handles database operations for groups
type GroupRepository struct {
	db *sqlx.DB
}

// NewGroupRepository creates a new GroupRepository
func NewGroupRepository(db *sqlx.DB) *GroupRepository {
	return &GroupRepository{db: db}
}

// Create inserts a new group and optionally creates auto admin group
func (r *GroupRepository) Create(req models.CreateGroupRequest) (*models.Group, error) {
	adminGroupID := req.AdminGroupID

	// If AdminGroupID not provided, we need to create group first, then auto-create admin group
	// But we can't do that without knowing the ID. So we require caller to handle this.
	// For now, just insert with null admin_group_id if not provided.

	query := `
		INSERT INTO groups (name, description, members_visible, allow_self_add, allow_sub_groups, admin_group_id)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	result, err := r.db.Exec(query, req.Name, req.Description, boolToInt(req.MembersVisible),
		boolToInt(req.AllowSelfAdd), boolToInt(req.AllowSubGroups), adminGroupID)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return r.GetByID(int(id))
}

// GetByID retrieves a group by ID
func (r *GroupRepository) GetByID(id int) (*models.Group, error) {
	var group models.Group
	query := `SELECT id, name, description, members_visible, allow_self_add, allow_sub_groups, admin_group_id FROM groups WHERE id = ?`
	err := r.db.Get(&group, query, id)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// GetByName retrieves a group by name
func (r *GroupRepository) GetByName(name string) (*models.Group, error) {
	var group models.Group
	query := `SELECT id, name, description, members_visible, allow_self_add, allow_sub_groups, admin_group_id FROM groups WHERE name = ?`
	err := r.db.Get(&group, query, name)
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// List returns all groups
func (r *GroupRepository) List() ([]models.Group, error) {
	var groups []models.Group
	query := `SELECT id, name, description, members_visible, allow_self_add, allow_sub_groups, admin_group_id FROM groups ORDER BY id`
	err := r.db.Select(&groups, query)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// Update modifies a group
func (r *GroupRepository) Update(id int, req models.UpdateGroupRequest) (*models.Group, error) {
	setParts := []string{}
	args := []interface{}{}

	if req.Name != nil {
		setParts = append(setParts, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Description != nil {
		setParts = append(setParts, "description = ?")
		args = append(args, *req.Description)
	}
	if req.MembersVisible != nil {
		setParts = append(setParts, "members_visible = ?")
		args = append(args, boolToInt(*req.MembersVisible))
	}
	if req.AllowSelfAdd != nil {
		setParts = append(setParts, "allow_self_add = ?")
		args = append(args, boolToInt(*req.AllowSelfAdd))
	}
	if req.AllowSubGroups != nil {
		setParts = append(setParts, "allow_sub_groups = ?")
		args = append(args, boolToInt(*req.AllowSubGroups))
	}
	if req.AdminGroupID != nil {
		setParts = append(setParts, "admin_group_id = ?")
		args = append(args, *req.AdminGroupID)
	}

	if len(setParts) == 0 {
		return r.GetByID(id)
	}

	query := "UPDATE groups SET " + setParts[0]
	for _, part := range setParts[1:] {
		query += ", " + part
	}
	query += " WHERE id = ?"
	args = append(args, id)

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

// Delete removes a group
func (r *GroupRepository) Delete(id int) error {
	_, err := r.db.Exec("DELETE FROM groups WHERE id = ?", id)
	return err
}

// AddPersonToGroup adds a person as member of a group, tracking who added them
func (r *GroupRepository) AddPersonToGroup(personID, groupID, addedBy int) error {
	query := `INSERT OR IGNORE INTO person_group_memberships (person_id, group_id, added_by, added_at) VALUES (?, ?, ?, datetime('now'))`
	_, err := r.db.Exec(query, personID, groupID, addedBy)
	return err
}

// RemovePersonFromGroup soft-deletes a person from a group by setting removed_at and removed_by
func (r *GroupRepository) RemovePersonFromGroup(personID, groupID, removedBy int) error {
	query := `UPDATE person_group_memberships SET removed_at = datetime('now'), removed_by = ? WHERE person_id = ? AND group_id = ? AND removed_at IS NULL`
	_, err := r.db.Exec(query, removedBy, personID, groupID)
	return err
}

// HardRemovePersonFromGroup permanently deletes a person from a group (use with caution)
func (r *GroupRepository) HardRemovePersonFromGroup(personID, groupID int) error {
	_, err := r.db.Exec("DELETE FROM person_group_memberships WHERE person_id = ? AND group_id = ?", personID, groupID)
	return err
}

// BulkOperationResult represents the result of a single bulk operation
type BulkOperationResult struct {
	PersonID int    `json:"person_id"`
	Success  bool   `json:"success"`
	Error    string `json:"error,omitempty"`
}

// BulkMembersRequest represents a bulk add/remove request
type BulkMembersRequest struct {
	PersonIDs []int  `json:"person_ids"`
	Action    string `json:"action"` // "add" or "remove"
}

// BulkMembersResponse represents the response from a bulk operation
type BulkMembersResponse struct {
	TotalRequested int                   `json:"total_requested"`
	TotalSuccess   int                   `json:"total_success"`
	TotalFailed    int                   `json:"total_failed"`
	Results        []BulkOperationResult `json:"results"`
}

// BulkAddMembersToGroup atomically adds multiple members to a group
func (r *GroupRepository) BulkAddMembersToGroup(personIDs []int, groupID, addedBy int) (*BulkMembersResponse, error) {
	response := &BulkMembersResponse{
		TotalRequested: len(personIDs),
		Results:        make([]BulkOperationResult, 0, len(personIDs)),
	}

	// Start a transaction
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, personID := range personIDs {
		result := BulkOperationResult{PersonID: personID}

		// Check if already a member (and not removed)
		var count int
		err := tx.Get(&count, "SELECT COUNT(1) FROM person_group_memberships WHERE person_id = ? AND group_id = ? AND removed_at IS NULL", personID, groupID)
		if err != nil {
			result.Success = false
			result.Error = "failed to check existing membership"
			response.Results = append(response.Results, result)
			response.TotalFailed++
			continue
		}

		if count > 0 {
			// Already a member - treat as success (idempotent)
			result.Success = true
			response.Results = append(response.Results, result)
			response.TotalSuccess++
			continue
		}

		// Check if was previously removed (re-adding)
		var removedCount int
		err = tx.Get(&removedCount, "SELECT COUNT(1) FROM person_group_memberships WHERE person_id = ? AND group_id = ? AND removed_at IS NOT NULL", personID, groupID)
		if err != nil {
			result.Success = false
			result.Error = "failed to check previous membership"
			response.Results = append(response.Results, result)
			response.TotalFailed++
			continue
		}

		if removedCount > 0 {
			// Re-activate by clearing removed_at and removed_by, update added_by
			_, err = tx.Exec("UPDATE person_group_memberships SET removed_at = NULL, removed_by = NULL, added_by = ?, added_at = datetime('now') WHERE person_id = ? AND group_id = ?", addedBy, personID, groupID)
		} else {
			// New insertion
			_, err = tx.Exec("INSERT INTO person_group_memberships (person_id, group_id, added_by, added_at) VALUES (?, ?, ?, datetime('now'))", personID, groupID, addedBy)
		}

		if err != nil {
			result.Success = false
			result.Error = "failed to add member"
			response.Results = append(response.Results, result)
			response.TotalFailed++
			continue
		}

		result.Success = true
		response.Results = append(response.Results, result)
		response.TotalSuccess++
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return response, nil
}

// BulkRemoveMembersFromGroup atomically removes multiple members from a group
func (r *GroupRepository) BulkRemoveMembersFromGroup(personIDs []int, groupID, removedBy int) (*BulkMembersResponse, error) {
	response := &BulkMembersResponse{
		TotalRequested: len(personIDs),
		Results:        make([]BulkOperationResult, 0, len(personIDs)),
	}

	// Start a transaction
	tx, err := r.db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	for _, personID := range personIDs {
		result := BulkOperationResult{PersonID: personID}

		// Check if actually a member
		var count int
		err := tx.Get(&count, "SELECT COUNT(1) FROM person_group_memberships WHERE person_id = ? AND group_id = ? AND removed_at IS NULL", personID, groupID)
		if err != nil {
			result.Success = false
			result.Error = "failed to check membership"
			response.Results = append(response.Results, result)
			response.TotalFailed++
			continue
		}

		if count == 0 {
			// Not a member - treat as success (idempotent)
			result.Success = true
			response.Results = append(response.Results, result)
			response.TotalSuccess++
			continue
		}

		// Perform soft delete
		_, err = tx.Exec("UPDATE person_group_memberships SET removed_at = datetime('now'), removed_by = ? WHERE person_id = ? AND group_id = ? AND removed_at IS NULL", removedBy, personID, groupID)
		if err != nil {
			result.Success = false
			result.Error = "failed to remove member"
			response.Results = append(response.Results, result)
			response.TotalFailed++
			continue
		}

		result.Success = true
		response.Results = append(response.Results, result)
		response.TotalSuccess++
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return response, nil
}

// AddSubgroup adds a group as subgroup of another group
func (r *GroupRepository) AddSubgroup(parentGroupID, childGroupID int) error {
	query := `INSERT OR IGNORE INTO group_subgroups (parent_group_id, child_group_id) VALUES (?, ?)`
	_, err := r.db.Exec(query, parentGroupID, childGroupID)
	return err
}

// RemoveSubgroup removes a subgroup relationship
func (r *GroupRepository) RemoveSubgroup(parentGroupID, childGroupID int) error {
	_, err := r.db.Exec("DELETE FROM group_subgroups WHERE parent_group_id = ? AND child_group_id = ?", parentGroupID, childGroupID)
	return err
}

// GetDirectPersonMembers returns person IDs directly in a group (excluding removed)
func (r *GroupRepository) GetDirectPersonMembers(groupID int) ([]int, error) {
	var ids []int
	query := `SELECT person_id FROM person_group_memberships WHERE group_id = ? AND removed_at IS NULL`
	err := r.db.Select(&ids, query, groupID)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// GetDirectPeopleAndSubgroups returns direct person IDs and subgroup IDs for a group
func (r *GroupRepository) GetDirectPeopleAndSubgroups(groupID int) ([]int, []int, error) {
	people, err := r.GetDirectPersonMembers(groupID)
	if err != nil {
		return nil, nil, err
	}

	groups, err := r.GetDirectSubgroups(groupID)
	if err != nil {
		return nil, nil, err
	}

	return people, groups, nil
}

// IsPersonDirectMember checks if person is directly a member of a group (and not removed)
func (r *GroupRepository) IsPersonDirectMember(personID, groupID int) (bool, error) {
	var count int
	query := `SELECT COUNT(1) FROM person_group_memberships WHERE person_id = ? AND group_id = ? AND removed_at IS NULL`
	if err := r.db.Get(&count, query, personID, groupID); err != nil {
		return false, err
	}
	return count > 0, nil
}

// IsPersonInGroup checks if person is a member of group, including through subgroups
func (r *GroupRepository) IsPersonInGroup(personID, groupID int) (bool, error) {
	participants, err := r.ResolveParticipants(groupID)
	if err != nil {
		return false, err
	}
	for _, pid := range participants {
		if pid == personID {
			return true, nil
		}
	}
	return false, nil
}

// IsPersonInAdminGroup checks if person is member of admin group (direct or indirect)
func (r *GroupRepository) IsPersonInAdminGroup(adminGroupID, personID int) (bool, error) {
	return r.IsPersonInGroup(personID, adminGroupID)
}


// GetDirectSubgroups returns child group IDs for a group
func (r *GroupRepository) GetDirectSubgroups(groupID int) ([]int, error) {
	var ids []int
	query := `SELECT child_group_id FROM group_subgroups WHERE parent_group_id = ?`
	err := r.db.Select(&ids, query, groupID)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// GetGroupsForPerson returns group IDs the person is directly in (excluding removed)
func (r *GroupRepository) GetGroupsForPerson(personID int) ([]int, error) {
	var ids []int
	query := `SELECT group_id FROM person_group_memberships WHERE person_id = ? AND removed_at IS NULL`
	err := r.db.Select(&ids, query, personID)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// GetParentGroups returns parent group IDs for a group (groups that have this as subgroup)
func (r *GroupRepository) GetParentGroups(groupID int) ([]int, error) {
	var ids []int
	query := `SELECT parent_group_id FROM group_subgroups WHERE child_group_id = ?`
	err := r.db.Select(&ids, query, groupID)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// ResolveParticipants returns all unique person IDs that are members of a group,
// recursively traversing all subgroups.
func (r *GroupRepository) ResolveParticipants(groupID int) ([]int, error) {
	seenGroups := make(map[int]bool)
	seenPeople := make(map[int]bool)
	people := []int{}

	r.resolveParticipantsRecursive(groupID, seenGroups, seenPeople, &people)

	return people, nil
}

func (r *GroupRepository) resolveParticipantsRecursive(groupID int, seenGroups map[int]bool, seenPeople map[int]bool, people *[]int) {
	// Avoid cycles in group traversal
	if seenGroups[groupID] {
		return
	}
	seenGroups[groupID] = true

	// Add direct person members (deduplicated)
	personIDs, err := r.GetDirectPersonMembers(groupID)
	if err == nil {
		for _, pid := range personIDs {
			if !seenPeople[pid] {
				seenPeople[pid] = true
				*people = append(*people, pid)
			}
		}
	}

	// Recurse into subgroups
	subgroupIDs, err := r.GetDirectSubgroups(groupID)
	if err == nil {
		for _, sid := range subgroupIDs {
			r.resolveParticipantsRecursive(sid, seenGroups, seenPeople, people)
		}
	}
}

// ResolveGroupsForPerson returns all unique group IDs a person is a member of,
// including transitive membership through parent groups.
func (r *GroupRepository) ResolveGroupsForPerson(personID int) ([]int, error) {
	seenGroups := make(map[int]bool)
	allGroups := []int{}

	// Start with direct groups
	directGroups, err := r.GetGroupsForPerson(personID)
	if err != nil {
		return nil, err
	}

	for _, gid := range directGroups {
		r.resolveGroupsRecursive(gid, seenGroups, &allGroups)
	}

	return allGroups, nil
}

func (r *GroupRepository) resolveGroupsRecursive(groupID int, seenGroups map[int]bool, allGroups *[]int) {
	// Avoid cycles
	if seenGroups[groupID] {
		return
	}
	seenGroups[groupID] = true
	*allGroups = append(*allGroups, groupID)

	// Add parent groups
	parentGroups, err := r.GetParentGroups(groupID)
	if err == nil {
		for _, pid := range parentGroups {
			r.resolveGroupsRecursive(pid, seenGroups, allGroups)
		}
	}
}

// Helper: bool to int conversion
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Helper: check if row exists
func rowExists(err error) bool {
	return err != sql.ErrNoRows && err != nil
}

// Search finds groups by substring match on name or description (LIKE fallback)
func (r *GroupRepository) Search(query string) ([]models.Group, error) {
	var groups []models.Group
	searchPattern := "%" + strings.ToLower(query) + "%"
	sqlQuery := `
		SELECT id, name, description, members_visible, allow_self_add, allow_sub_groups, admin_group_id
		FROM groups
		WHERE LOWER(name) LIKE ? OR LOWER(description) LIKE ?
		ORDER BY id
	`
	err := r.db.Select(&groups, sqlQuery, searchPattern, searchPattern)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// SearchFTS searches groups using FTS5 for faster, more accurate results
func (r *GroupRepository) SearchFTS(query string) ([]models.Group, error) {
	var groups []models.Group
	sqlQuery := `
		SELECT g.id, g.name, g.description, g.members_visible, g.allow_self_add, g.allow_sub_groups, g.admin_group_id
		FROM groups g
		JOIN groups_fts ON g.id = groups_fts.rowid
		WHERE groups_fts MATCH ?
		ORDER BY bm25(groups_fts, 1.0, 0.5)
		LIMIT 50
	`
	err := r.db.Select(&groups, sqlQuery, query)
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// SearchWithFallback tries FTS search first, falls back to LIKE if FTS unavailable
func (r *GroupRepository) SearchWithFallback(query string, useFTS bool) ([]models.Group, error) {
	if useFTS {
		groups, err := r.SearchFTS(query)
		if err == nil && len(groups) > 0 {
			return groups, nil
		}
	}
	return r.Search(query)
}

// SearchResult represents a unified search result for the combined endpoint
type SearchResult struct {
	People []models.Person `json:"people"`
	Groups []models.Group  `json:"groups"`
}

// SearchAll performs unified search across people and groups
func (r *GroupRepository) SearchAll(query string, personRepo *PersonRepository, useFTS bool) (*SearchResult, error) {
	people, err := personRepo.SearchWithFallback(query, useFTS)
	if err != nil {
		people = []models.Person{}
	}

	groups, err := r.SearchWithFallback(query, useFTS)
	if err != nil {
		groups = []models.Group{}
	}

	return &SearchResult{
		People: people,
		Groups: groups,
	}, nil
}

// MembershipActivity represents a single membership change (addition or removal)
type MembershipActivity struct {
	PersonID   int        `db:"person_id" json:"person_id"`
	GroupID    int        `db:"group_id" json:"group_id"`
	AddedBy    *int       `db:"added_by" json:"added_by,omitempty"`
	AddedAt    *string    `db:"added_at" json:"added_at,omitempty"`
	RemovedAt  *string    `db:"removed_at" json:"removed_at,omitempty"`
	ActivityType string   `json:"activity_type"` // "added" or "removed"
}

// GetMembershipActivitiesFromPastDay returns all membership additions and removals from the past 24 hours
// Results are grouped by person_id in a map
func (r *GroupRepository) GetMembershipActivitiesFromPastDay() (map[int][]MembershipActivity, error) {
	// Get all additions from past day
	query := `
		SELECT person_id, group_id, added_by, added_at,
			CASE WHEN removed_at IS NULL THEN NULL ELSE removed_at END as removed_at
		FROM person_group_memberships
		WHERE (added_at >= datetime('now', '-1 day'))
		   OR (removed_at >= datetime('now', '-1 day'))
		ORDER BY person_id, added_at, removed_at
	`

	var rows []struct {
		PersonID  int     `db:"person_id"`
		GroupID   int     `db:"group_id"`
		AddedBy   *int    `db:"added_by"`
		AddedAt   *string `db:"added_at"`
		RemovedAt *string `db:"removed_at"`
	}

	err := r.db.Select(&rows, query)
	if err != nil {
		return nil, err
	}

	// Group by person_id
	activitiesByPerson := make(map[int][]MembershipActivity)

	for _, row := range rows {
		// Check if this is a new addition (added in past day)
		if row.AddedAt != nil {
			activity := MembershipActivity{
				PersonID:     row.PersonID,
				GroupID:      row.GroupID,
				AddedBy:      row.AddedBy,
				AddedAt:      row.AddedAt,
				ActivityType: "added",
			}
			activitiesByPerson[row.PersonID] = append(activitiesByPerson[row.PersonID], activity)
		}

		// Check if this is a removal (removed in past day)
		if row.RemovedAt != nil {
			activity := MembershipActivity{
				PersonID:     row.PersonID,
				GroupID:      row.GroupID,
				RemovedAt:    row.RemovedAt,
				ActivityType: "removed",
			}
			activitiesByPerson[row.PersonID] = append(activitiesByPerson[row.PersonID], activity)
		}
	}

	return activitiesByPerson, nil
}

// GetMembershipReport generates a report of all membership changes in the past day
// Returns a map of person_id to their list of activities
func (r *GroupRepository) GetMembershipReport() (map[int][]MembershipActivity, error) {
	return r.GetMembershipActivitiesFromPastDay()
}
