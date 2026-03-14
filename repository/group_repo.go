package repository

import (
	"database/sql"

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
	query := `INSERT OR IGNORE INTO person_group_memberships (person_id, group_id, added_by) VALUES (?, ?, ?)`
	_, err := r.db.Exec(query, personID, groupID, addedBy)
	return err
}

// RemovePersonFromGroup removes a person from a group
func (r *GroupRepository) RemovePersonFromGroup(personID, groupID int) error {
	_, err := r.db.Exec("DELETE FROM person_group_memberships WHERE person_id = ? AND group_id = ?", personID, groupID)
	return err
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

// GetDirectPersonMembers returns person IDs directly in a group
func (r *GroupRepository) GetDirectPersonMembers(groupID int) ([]int, error) {
	var ids []int
	query := `SELECT person_id FROM person_group_memberships WHERE group_id = ?`
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

// IsPersonDirectMember checks if person is directly a member of a group
func (r *GroupRepository) IsPersonDirectMember(personID, groupID int) (bool, error) {
	var count int
	query := `SELECT COUNT(1) FROM person_group_memberships WHERE person_id = ? AND group_id = ?`
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

// GetGroupsForPerson returns group IDs the person is directly in
func (r *GroupRepository) GetGroupsForPerson(personID int) ([]int, error) {
	var ids []int
	query := `SELECT group_id FROM person_group_memberships WHERE person_id = ?`
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
