package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gorilla/mux"

	"github.com/umakantv/people/models"
	"github.com/umakantv/people/repository"
	"github.com/umakantv/people/testhelpers"
)

func setupGroupHandler(t *testing.T) (*GroupHandler, *repository.GroupRepository, *repository.PersonRepository) {
	t.Helper()
	db := testhelpers.SetupTestDB(t)
	t.Cleanup(func() { testhelpers.CloseDB(t, db) })
	personRepo := repository.NewPersonRepository(db)
	groupRepo := repository.NewGroupRepository(db)
	return NewGroupHandler(groupRepo, personRepo), groupRepo, personRepo
}

func createPerson(t *testing.T, personRepo *repository.PersonRepository, name, email string) *models.Person {
	t.Helper()
	p, err := personRepo.Create(models.CreatePersonRequest{Name: name, Email: email, JoinedDate: "2024-01-01"})
	if err != nil {
		t.Fatalf("failed to create person: %v", err)
	}
	return p
}

func TestGroupCreate_SuccessAutoAdmin(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")

	body := `{"name":"Developers","description":"Dev group"}`
	req := httptest.NewRequest(http.MethodPost, "/groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	rec := httptest.NewRecorder()

	handler.Create(context.Background(), rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var group models.Group
	json.Unmarshal(rec.Body.Bytes(), &group)

	if group.AdminGroupID == nil {
		t.Fatalf("expected admin_group_id to be set")
	}

	// Verify admin group exists
	adminGroup, err := groupRepo.GetByID(*group.AdminGroupID)
	if err != nil {
		t.Fatalf("admin group not found: %v", err)
	}
	if adminGroup.Name != "Developers-Admins" {
		t.Errorf("expected admin group name 'Developers-Admins', got '%s'", adminGroup.Name)
	}

	// Requestor should be member of group and admin group
	isMember, _ := groupRepo.IsPersonInGroup(creator.ID, group.ID)
	if !isMember {
		t.Errorf("requestor should be member of group")
	}
	isAdmin, _ := groupRepo.IsPersonInGroup(creator.ID, *group.AdminGroupID)
	if !isAdmin {
		t.Errorf("requestor should be member of admin group")
	}
}

func TestGroupCreate_WithProvidedAdminGroup(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	adminGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Admins"})

	body := `{"name":"QA","admin_group_id":` + itoa(adminGroup.ID) + `}`
	req := httptest.NewRequest(http.MethodPost, "/groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	rec := httptest.NewRecorder()

	handler.Create(context.Background(), rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rec.Code)
	}

	var group models.Group
	json.Unmarshal(rec.Body.Bytes(), &group)
	if group.AdminGroupID == nil || *group.AdminGroupID != adminGroup.ID {
		t.Errorf("expected admin_group_id to match provided group")
	}

	// Creator should be added to admin group and new group
	isAdmin, _ := groupRepo.IsPersonInGroup(creator.ID, adminGroup.ID)
	if !isAdmin {
		t.Errorf("creator should be in provided admin group")
	}
}

func TestGroupCreate_MissingRequestor(t *testing.T) {
	handler, _, _ := setupGroupHandler(t)

	body := `{"name":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/groups", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.Create(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing X-Person-Id, got %d", rec.Code)
	}
}

func TestGroupCreate_RequestorNotFound(t *testing.T) {
	handler, _, _ := setupGroupHandler(t)

	body := `{"name":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/groups", bytes.NewBufferString(body))
	req.Header.Set("X-Person-Id", "999")
	rec := httptest.NewRecorder()

	handler.Create(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown requestor, got %d", rec.Code)
	}
}

func TestGroupCreate_InvalidBody(t *testing.T) {
	handler, _, personRepo := setupGroupHandler(t)

	createPerson(t, personRepo, "Alice", "alice@test.com")

	req := httptest.NewRequest(http.MethodPost, "/groups", bytes.NewBufferString("not-json"))
	req.Header.Set("X-Person-Id", "1")
	rec := httptest.NewRecorder()

	handler.Create(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid json, got %d", rec.Code)
	}
}

func TestGroupUpdate_Success(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")

	// Create group with admin group
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Update members_visible
	body := `{"members_visible":true}`
	req := httptest.NewRequest(http.MethodPut, "/groups/"+itoa(group.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.Update(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var updated models.Group
	json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.MembersVisible != 1 {
		t.Errorf("expected members_visible=1")
	}
}

func TestGroupUpdate_NotAdmin(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	other := createPerson(t, personRepo, "Bob", "bob@test.com")

	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	body := `{"members_visible":true}`
	req := httptest.NewRequest(http.MethodPut, "/groups/"+itoa(group.ID), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(other.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.Update(context.Background(), rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", rec.Code)
	}
}

func TestGroupUpdate_GroupNotFound(t *testing.T) {
	handler, _, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")

	body := `{"members_visible":true}`
	req := httptest.NewRequest(http.MethodPut, "/groups/999", bytes.NewBufferString(body))
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	rec := httptest.NewRecorder()

	handler.Update(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGroupAddMember_Success(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	member := createPerson(t, personRepo, "Bob", "bob@test.com")

	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	body := `{"person_id":` + itoa(member.ID) + `}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.AddMember(context.Background(), rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", rec.Code)
	}

	isMember, _ := groupRepo.IsPersonInGroup(member.ID, group.ID)
	if !isMember {
		t.Errorf("member should be in group")
	}
}

func TestGroupAddMember_NotAdmin(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	member := createPerson(t, personRepo, "Bob", "bob@test.com")
	other := createPerson(t, personRepo, "Carol", "carol@test.com")

	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	body := `{"person_id":` + itoa(member.ID) + `}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(other.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.AddMember(context.Background(), rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestGroupAddMember_PersonNotFound(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	body := `{"person_id":999}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.AddMember(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGroupDirectMembers_Success(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	member := createPerson(t, personRepo, "Bob", "bob@test.com")

	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")
	childGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Child"})

	groupRepo.AddPersonToGroup(member.ID, group.ID, creator.ID)
	groupRepo.AddSubgroup(group.ID, childGroup.ID)

	req := httptest.NewRequest(http.MethodGet, "/groups/"+itoa(group.ID)+"/members/direct", nil)
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.GetDirectMembers(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string][]int
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp["people"]) != 2 { // creator + member
		t.Errorf("expected 2 direct people, got %d", len(resp["people"]))
	}
	if len(resp["subgroups"]) != 1 {
		t.Errorf("expected 1 subgroup, got %d", len(resp["subgroups"]))
	}
}

func TestGroupDirectMembers_GroupNotFound(t *testing.T) {
	handler, _, _ := setupGroupHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/groups/999/members/direct", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "999"})
	rec := httptest.NewRecorder()

	handler.GetDirectMembers(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGroupIsMember_Success(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	member := createPerson(t, personRepo, "Bob", "bob@test.com")

	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")
	groupRepo.AddPersonToGroup(member.ID, group.ID, creator.ID)

	req := httptest.NewRequest(http.MethodGet, "/groups/"+itoa(group.ID)+"/members/"+itoa(member.ID)+"/check", nil)
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID), "personId": itoa(member.ID)})
	rec := httptest.NewRecorder()

	handler.IsMember(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]bool
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp["is_member"] {
		t.Errorf("expected is_member=true")
	}
}

func TestGroupIsMember_False(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	member := createPerson(t, personRepo, "Bob", "bob@test.com")

	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	req := httptest.NewRequest(http.MethodGet, "/groups/"+itoa(group.ID)+"/members/"+itoa(member.ID)+"/check", nil)
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID), "personId": itoa(member.ID)})
	rec := httptest.NewRecorder()

	handler.IsMember(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var resp map[string]bool
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["is_member"] {
		t.Errorf("expected is_member=false")
	}
}

func TestGroupIsMember_GroupNotFound(t *testing.T) {
	handler, _, _ := setupGroupHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/groups/999/members/1/check", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "999", "personId": "1"})
	rec := httptest.NewRecorder()

	handler.IsMember(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGroupIsMember_InvalidPersonID(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	req := httptest.NewRequest(http.MethodGet, "/groups/"+itoa(group.ID)+"/members/abc/check", nil)
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID), "personId": "abc"})
	rec := httptest.NewRecorder()

	handler.IsMember(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

// Helpers
func createGroupWithAdmin(t *testing.T, handler *GroupHandler, groupRepo *repository.GroupRepository, requestorID int, name string) *models.Group {
	t.Helper()
	body := `{"name":"` + name + `"}`
	req := httptest.NewRequest(http.MethodPost, "/groups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(requestorID))
	rec := httptest.NewRecorder()

	handler.Create(context.Background(), rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected create group 201, got %d", rec.Code)
	}

	var group models.Group
	json.Unmarshal(rec.Body.Bytes(), &group)

	return &group
}

func itoa(id int) string {
	return strconv.Itoa(id)
}

func TestGroupAddSubgroup_Success(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")

	// Create parent group with allow_sub_groups=true
	parentGroup := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Parent")
	// Enable subgroups
	groupRepo.Update(parentGroup.ID, models.UpdateGroupRequest{AllowSubGroups: boolPtr(true)})

	// Create child group
	childGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Child"})

	body := `{"subgroup_id":` + itoa(childGroup.ID) + `}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(parentGroup.ID)+"/subgroups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(parentGroup.ID)})
	rec := httptest.NewRecorder()

	handler.AddSubgroup(context.Background(), rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["status"] != "added" {
		t.Errorf("expected status=added")
	}
}

func TestGroupAddSubgroup_SubgroupsNotAllowed(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")

	// Create parent group without allow_sub_groups
	parentGroup := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Parent")
	// Ensure subgroups NOT allowed (default is false)

	childGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Child"})

	body := `{"subgroup_id":` + itoa(childGroup.ID) + `}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(parentGroup.ID)+"/subgroups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(parentGroup.ID)})
	rec := httptest.NewRecorder()

	handler.AddSubgroup(context.Background(), rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for subgroups not allowed, got %d", rec.Code)
	}
}

func TestGroupAddSubgroup_NotAdmin(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	other := createPerson(t, personRepo, "Bob", "bob@test.com")

	// Create parent group with allow_sub_groups
	parentGroup := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Parent")
	groupRepo.Update(parentGroup.ID, models.UpdateGroupRequest{AllowSubGroups: boolPtr(true)})

	childGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Child"})

	body := `{"subgroup_id":` + itoa(childGroup.ID) + `}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(parentGroup.ID)+"/subgroups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(other.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(parentGroup.ID)})
	rec := httptest.NewRecorder()

	handler.AddSubgroup(context.Background(), rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", rec.Code)
	}
}

func TestGroupAddSubgroup_SelfReference(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")

	parentGroup := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Parent")
	groupRepo.Update(parentGroup.ID, models.UpdateGroupRequest{AllowSubGroups: boolPtr(true)})

	body := `{"subgroup_id":` + itoa(parentGroup.ID) + `}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(parentGroup.ID)+"/subgroups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(parentGroup.ID)})
	rec := httptest.NewRecorder()

	handler.AddSubgroup(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for self-reference, got %d", rec.Code)
	}
}

func TestGroupAddSubgroup_CircularReference(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")

	// Create two groups with allow_sub_groups
	parentGroup := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Parent")
	groupRepo.Update(parentGroup.ID, models.UpdateGroupRequest{AllowSubGroups: boolPtr(true)})

	childGroup := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Child")
	groupRepo.Update(childGroup.ID, models.UpdateGroupRequest{AllowSubGroups: boolPtr(true)})

	// First add child as subgroup of parent
	groupRepo.AddSubgroup(parentGroup.ID, childGroup.ID)

	// Now try to add parent as subgroup of child (would create cycle)
	body := `{"subgroup_id":` + itoa(parentGroup.ID) + `}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(childGroup.ID)+"/subgroups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(childGroup.ID)})
	rec := httptest.NewRecorder()

	handler.AddSubgroup(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for circular reference, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGroupAddSubgroup_SubgroupNotFound(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")

	parentGroup := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Parent")
	groupRepo.Update(parentGroup.ID, models.UpdateGroupRequest{AllowSubGroups: boolPtr(true)})

	body := `{"subgroup_id":999}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(parentGroup.ID)+"/subgroups", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(parentGroup.ID)})
	rec := httptest.NewRecorder()

	handler.AddSubgroup(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for subgroup not found, got %d", rec.Code)
	}
}

func TestGroupRemoveSubgroup_Success(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")

	parentGroup := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Parent")
	childGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Child"})

	// Add subgroup first
	groupRepo.AddSubgroup(parentGroup.ID, childGroup.ID)

	req := httptest.NewRequest(http.MethodDelete, "/groups/"+itoa(parentGroup.ID)+"/subgroups/"+itoa(childGroup.ID), nil)
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(parentGroup.ID), "subgroupId": itoa(childGroup.ID)})
	rec := httptest.NewRecorder()

	handler.RemoveSubgroup(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["status"] != "removed" {
		t.Errorf("expected status=removed")
	}
}

func TestGroupRemoveSubgroup_NotAdmin(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	other := createPerson(t, personRepo, "Bob", "bob@test.com")

	parentGroup := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Parent")
	childGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Child"})

	groupRepo.AddSubgroup(parentGroup.ID, childGroup.ID)

	req := httptest.NewRequest(http.MethodDelete, "/groups/"+itoa(parentGroup.ID)+"/subgroups/"+itoa(childGroup.ID), nil)
	req.Header.Set("X-Person-Id", itoa(other.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(parentGroup.ID), "subgroupId": itoa(childGroup.ID)})
	rec := httptest.NewRecorder()

	handler.RemoveSubgroup(context.Background(), rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", rec.Code)
	}
}

func TestGroupRemoveSubgroup_GroupNotFound(t *testing.T) {
	handler, _, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")

	req := httptest.NewRequest(http.MethodDelete, "/groups/999/subgroups/1", nil)
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": "999", "subgroupId": "1"})
	rec := httptest.NewRecorder()

	handler.RemoveSubgroup(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestGroupRemoveMember_Success(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	// Create a group (creator becomes admin)
	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Add a member to the group
	member := createPerson(t, personRepo, "Bob", "bob@test.com")
	groupRepo.AddPersonToGroup(member.ID, group.ID, creator.ID)

	// Verify member is in group
	isMember, _ := groupRepo.IsPersonDirectMember(member.ID, group.ID)
	if !isMember {
		t.Fatalf("member should be in group before removal")
	}

	// Admin removes member
	req := httptest.NewRequest(http.MethodDelete, "/groups/"+itoa(group.ID)+"/members/"+itoa(member.ID), nil)
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID), "personId": itoa(member.ID)})
	rec := httptest.NewRecorder()

	handler.RemoveMember(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["status"] != "removed" {
		t.Errorf("expected status=removed")
	}

	// Verify member is no longer in group
	isMember, _ = groupRepo.IsPersonDirectMember(member.ID, group.ID)
	if isMember {
		t.Errorf("member should be removed from group")
	}
}

func TestGroupRemoveMember_NotAdmin(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	// Create a group (creator becomes admin)
	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Add two members
	member1 := createPerson(t, personRepo, "Bob", "bob@test.com")
	member2 := createPerson(t, personRepo, "Carol", "carol@test.com")
	groupRepo.AddPersonToGroup(member1.ID, group.ID, creator.ID)
	groupRepo.AddPersonToGroup(member2.ID, group.ID, creator.ID)

	// Non-admin tries to remove another member
	req := httptest.NewRequest(http.MethodDelete, "/groups/"+itoa(group.ID)+"/members/"+itoa(member2.ID), nil)
	req.Header.Set("X-Person-Id", itoa(member1.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID), "personId": itoa(member2.ID)})
	rec := httptest.NewRecorder()

	handler.RemoveMember(context.Background(), rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", rec.Code)
	}
}

func TestGroupRemoveMember_SelfRemoval(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	// Create a group (creator becomes admin)
	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Admin tries to remove themselves
	req := httptest.NewRequest(http.MethodDelete, "/groups/"+itoa(group.ID)+"/members/"+itoa(creator.ID), nil)
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID), "personId": itoa(creator.ID)})
	rec := httptest.NewRecorder()

	handler.RemoveMember(context.Background(), rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for self-removal, got %d", rec.Code)
	}
}

func TestGroupRemoveMember_RemoveAdmin(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	// Create a group (creator becomes admin)
	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Add another person to admin group (making them also an admin)
	admin2 := createPerson(t, personRepo, "Bob", "bob@test.com")
	// Get admin group ID from the created group
	updatedGroup, _ := groupRepo.GetByID(group.ID)
	groupRepo.AddPersonToGroup(admin2.ID, *updatedGroup.AdminGroupID, creator.ID)

	// First admin tries to remove second admin
	req := httptest.NewRequest(http.MethodDelete, "/groups/"+itoa(group.ID)+"/members/"+itoa(admin2.ID), nil)
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID), "personId": itoa(admin2.ID)})
	rec := httptest.NewRecorder()

	handler.RemoveMember(context.Background(), rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for removing another admin, got %d", rec.Code)
	}
}

func TestGroupRemoveMember_PersonNotFound(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	// Create a group (creator becomes admin)
	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Try to remove non-existent person
	req := httptest.NewRequest(http.MethodDelete, "/groups/"+itoa(group.ID)+"/members/999", nil)
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID), "personId": "999"})
	rec := httptest.NewRecorder()

	handler.RemoveMember(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func boolPtr(b bool) *bool {
	return &b
}

// Bulk Operations Tests

func TestGroupBulkMembers_AddSuccess(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	// Create group (creator becomes admin)
	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Create people to add
	p1 := createPerson(t, personRepo, "Bob", "bob@test.com")
	p2 := createPerson(t, personRepo, "Carol", "carol@test.com")
	p3 := createPerson(t, personRepo, "Dave", "dave@test.com")

	// Bulk add
	body := fmt.Sprintf(`{"person_ids": [%d, %d, %d], "action": "add"}`, p1.ID, p2.ID, p3.ID)
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp repository.BulkMembersResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.TotalRequested != 3 {
		t.Errorf("expected total_requested=3, got %d", resp.TotalRequested)
	}
	if resp.TotalSuccess != 3 {
		t.Errorf("expected total_success=3, got %d", resp.TotalSuccess)
	}
	if resp.TotalFailed != 0 {
		t.Errorf("expected total_failed=0, got %d", resp.TotalFailed)
	}

	// Verify all members were added
	for _, p := range []*models.Person{p1, p2, p3} {
		isMember, _ := groupRepo.IsPersonDirectMember(p.ID, group.ID)
		if !isMember {
			t.Errorf("person %d should be member", p.ID)
		}
	}
}

func TestGroupBulkMembers_RemoveSuccess(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	// Create group (creator becomes admin)
	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Create and add people
	p1 := createPerson(t, personRepo, "Bob", "bob@test.com")
	p2 := createPerson(t, personRepo, "Carol", "carol@test.com")
	groupRepo.AddPersonToGroup(p1.ID, group.ID, creator.ID)
	groupRepo.AddPersonToGroup(p2.ID, group.ID, creator.ID)

	// Bulk remove
	body := fmt.Sprintf(`{"person_ids": [%d, %d], "action": "remove"}`, p1.ID, p2.ID)
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp repository.BulkMembersResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.TotalSuccess != 2 {
		t.Errorf("expected total_success=2, got %d", resp.TotalSuccess)
	}

	// Verify all members were removed
	for _, p := range []*models.Person{p1, p2} {
		isMember, _ := groupRepo.IsPersonDirectMember(p.ID, group.ID)
		if isMember {
			t.Errorf("person %d should not be member", p.ID)
		}
	}
}

func TestGroupBulkMembers_EmptyArray(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	body := `{"person_ids": [], "action": "add"}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty array, got %d", rec.Code)
	}
}

func TestGroupBulkMembers_TooManyIDs(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Create array with 101 IDs
	ids := make([]int, 101)
	for i := range ids {
		ids[i] = i + 1
	}
	body := fmt.Sprintf(`{"person_ids": %v, "action": "add"}`, ids)
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for too many IDs, got %d", rec.Code)
	}
}

func TestGroupBulkMembers_InvalidAction(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	p1 := createPerson(t, personRepo, "Bob", "bob@test.com")

	body := fmt.Sprintf(`{"person_ids": [%d], "action": "invalid"}`, p1.ID)
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid action, got %d", rec.Code)
	}
}

func TestGroupBulkMembers_NotAdmin(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Non-admin member
	member := createPerson(t, personRepo, "Bob", "bob@test.com")
	groupRepo.AddPersonToGroup(member.ID, group.ID, creator.ID)

	p2 := createPerson(t, personRepo, "Carol", "carol@test.com")

	body := fmt.Sprintf(`{"person_ids": [%d], "action": "add"}`, p2.ID)
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(member.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-admin, got %d", rec.Code)
	}
}

func TestGroupBulkMembers_InvalidPersonID(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Try to add non-existent person
	body := `{"person_ids": [99999], "action": "add"}`
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for invalid person ID, got %d", rec.Code)
	}
}

func TestGroupBulkMembers_IdempotentAdd(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	p1 := createPerson(t, personRepo, "Bob", "bob@test.com")
	groupRepo.AddPersonToGroup(p1.ID, group.ID, creator.ID) // Already a member

	// Try to add again
	body := fmt.Sprintf(`{"person_ids": [%d], "action": "add"}`, p1.ID)
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp repository.BulkMembersResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.TotalSuccess != 1 {
		t.Errorf("expected total_success=1 for idempotent add, got %d", resp.TotalSuccess)
	}
}

func TestGroupBulkMembers_IdempotentRemove(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	p1 := createPerson(t, personRepo, "Bob", "bob@test.com")
	// p1 is NOT a member

	// Try to remove (should succeed idempotently)
	body := fmt.Sprintf(`{"person_ids": [%d], "action": "remove"}`, p1.ID)
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp repository.BulkMembersResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.TotalSuccess != 1 {
		t.Errorf("expected total_success=1 for idempotent remove, got %d", resp.TotalSuccess)
	}
}

func TestGroupBulkMembers_LargeBatch(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	// Create 50 people
	var ids []int
	for i := 0; i < 50; i++ {
		p := createPerson(t, personRepo, fmt.Sprintf("Person%d", i), fmt.Sprintf("p%d@test.com", i))
		ids = append(ids, p.ID)
	}

	// Build JSON array
	idsStr := "["
	for i, id := range ids {
		if i > 0 {
			idsStr += ","
		}
		idsStr += itoa(id)
	}
	idsStr += "]"

	body := fmt.Sprintf(`{"person_ids": %s, "action": "add"}`, idsStr)
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp repository.BulkMembersResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp.TotalSuccess != 50 {
		t.Errorf("expected total_success=50, got %d", resp.TotalSuccess)
	}

	// Verify all members were added
	for _, id := range ids {
		isMember, _ := groupRepo.IsPersonDirectMember(id, group.ID)
		if !isMember {
			t.Errorf("person %d should be member", id)
		}
	}
}

func TestGroupBulkMembers_ReAddRemovedMember(t *testing.T) {
	handler, groupRepo, personRepo := setupGroupHandler(t)

	creator := createPerson(t, personRepo, "Alice", "alice@test.com")
	group := createGroupWithAdmin(t, handler, groupRepo, creator.ID, "Team")

	p1 := createPerson(t, personRepo, "Bob", "bob@test.com")

	// Add, remove, then re-add
	groupRepo.AddPersonToGroup(p1.ID, group.ID, creator.ID)
	groupRepo.RemovePersonFromGroup(p1.ID, group.ID, creator.ID)

	// Re-add via bulk
	body := fmt.Sprintf(`{"person_ids": [%d], "action": "add"}`, p1.ID)
	req := httptest.NewRequest(http.MethodPost, "/groups/"+itoa(group.ID)+"/bulk-members", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Person-Id", itoa(creator.ID))
	req = mux.SetURLVars(req, map[string]string{"id": itoa(group.ID)})
	rec := httptest.NewRecorder()

	handler.BulkMembers(context.Background(), rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify member is back
	isMember, _ := groupRepo.IsPersonDirectMember(p1.ID, group.ID)
	if !isMember {
		t.Errorf("person should be re-added as member")
	}
}
