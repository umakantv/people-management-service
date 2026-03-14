package models

// Group represents a group entity
type Group struct {
	ID              int    `json:"id" db:"id"`
	Name            string `json:"name" db:"name"`
	Description     string `json:"description" db:"description"`
	MembersVisible  int    `json:"members_visible" db:"members_visible"`
	AllowSelfAdd    int    `json:"allow_self_add" db:"allow_self_add"`
	AllowSubGroups  int    `json:"allow_sub_groups" db:"allow_sub_groups"`
	AdminGroupID    *int   `json:"admin_group_id,omitempty" db:"admin_group_id"`
}

// CreateGroupRequest represents the request body for creating a group
type CreateGroupRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	MembersVisible bool   `json:"members_visible,omitempty"`
	AllowSelfAdd   bool   `json:"allow_self_add,omitempty"`
	AllowSubGroups bool   `json:"allow_sub_groups,omitempty"`
	AdminGroupID   *int   `json:"admin_group_id,omitempty"`
}

// UpdateGroupRequest represents the request body for updating a group
type UpdateGroupRequest struct {
	Name           *string `json:"name,omitempty"`
	Description    *string `json:"description,omitempty"`
	MembersVisible *bool   `json:"members_visible,omitempty"`
	AllowSelfAdd   *bool   `json:"allow_self_add,omitempty"`
	AllowSubGroups *bool   `json:"allow_sub_groups,omitempty"`
	AdminGroupID   *int    `json:"admin_group_id,omitempty"`
}
