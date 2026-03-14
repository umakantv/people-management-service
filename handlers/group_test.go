package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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
