package repository

import (
	"testing"

	"github.com/umakantv/people/models"
	"github.com/umakantv/people/testhelpers"
)

// setupGroupTest creates a GroupRepository with in-memory DB and some test data
func setupGroupTest(t *testing.T) (*GroupRepository, *PersonRepository) {
	t.Helper()
	db := testhelpers.SetupTestDB(t)
	t.Cleanup(func() { testhelpers.CloseDB(t, db) })
	return NewGroupRepository(db), NewPersonRepository(db)
}

func TestResolveParticipants_SimpleGroup(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	// Create people
	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})
	p2, _ := personRepo.Create(models.CreatePersonRequest{Name: "Bob", Email: "b@test.com", JoinedDate: "2024-01-01"})

	// Create group
	group, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Team A"})

	// Add people to group
	groupRepo.AddPersonToGroup(p1.ID, group.ID, p1.ID)
	groupRepo.AddPersonToGroup(p2.ID, group.ID, p2.ID)

	// Resolve
	participants, err := groupRepo.ResolveParticipants(group.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(participants) != 2 {
		t.Errorf("expected 2 participants, got %d", len(participants))
	}
}

func TestResolveParticipants_WithSubgroups(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	// Create people
	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})
	p2, _ := personRepo.Create(models.CreatePersonRequest{Name: "Bob", Email: "b@test.com", JoinedDate: "2024-01-01"})
	p3, _ := personRepo.Create(models.CreatePersonRequest{Name: "Charlie", Email: "c@test.com", JoinedDate: "2024-01-01"})

	// Create groups
	parentGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Parent", AllowSubGroups: true})
	childGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Child"})

	// Parent has p1 directly
	groupRepo.AddPersonToGroup(p1.ID, parentGroup.ID, p1.ID)

	// Child has p2, p3
	groupRepo.AddPersonToGroup(p2.ID, childGroup.ID, p2.ID)
	groupRepo.AddPersonToGroup(p3.ID, childGroup.ID, p3.ID)

	// Parent has Child as subgroup
	groupRepo.AddSubgroup(parentGroup.ID, childGroup.ID)

	// Resolve participants of parent
	participants, err := groupRepo.ResolveParticipants(parentGroup.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have p1 (direct) + p2, p3 (from subgroup) = 3 unique
	if len(participants) != 3 {
		t.Errorf("expected 3 participants, got %d: %v", len(participants), participants)
	}
}

func TestResolveParticipants_Deduplication(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})

	// Person in both parent and child group
	parentGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Parent", AllowSubGroups: true})
	childGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Child"})

	groupRepo.AddPersonToGroup(p1.ID, parentGroup.ID, p1.ID)
	groupRepo.AddPersonToGroup(p1.ID, childGroup.ID, p1.ID)
	groupRepo.AddSubgroup(parentGroup.ID, childGroup.ID)

	participants, _ := groupRepo.ResolveParticipants(parentGroup.ID)

	// p1 should appear only once
	if len(participants) != 1 {
		t.Errorf("expected 1 unique participant, got %d", len(participants))
	}
}

func TestResolveParticipants_CircularReference(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})

	// Create circular: A -> B -> A
	groupA, _ := groupRepo.Create(models.CreateGroupRequest{Name: "A", AllowSubGroups: true})
	groupB, _ := groupRepo.Create(models.CreateGroupRequest{Name: "B", AllowSubGroups: true})

	groupRepo.AddPersonToGroup(p1.ID, groupA.ID, p1.ID)
	groupRepo.AddSubgroup(groupA.ID, groupB.ID)
	groupRepo.AddSubgroup(groupB.ID, groupA.ID) // Circular!

	// Should not infinite loop
	participants, err := groupRepo.ResolveParticipants(groupA.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(participants) != 1 {
		t.Errorf("expected 1 participant despite circular ref, got %d", len(participants))
	}
}

func TestResolveParticipants_DeepNesting(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})

	// Create deep nesting: A -> B -> C -> person
	groupA, _ := groupRepo.Create(models.CreateGroupRequest{Name: "A", AllowSubGroups: true})
	groupB, _ := groupRepo.Create(models.CreateGroupRequest{Name: "B", AllowSubGroups: true})
	groupC, _ := groupRepo.Create(models.CreateGroupRequest{Name: "C"})

	groupRepo.AddPersonToGroup(p1.ID, groupC.ID, p1.ID)
	groupRepo.AddSubgroup(groupA.ID, groupB.ID)
	groupRepo.AddSubgroup(groupB.ID, groupC.ID)

	participants, _ := groupRepo.ResolveParticipants(groupA.ID)

	if len(participants) != 1 {
		t.Errorf("expected 1 participant from deep nesting, got %d", len(participants))
	}
}

func TestResolveGroupsForPerson_Simple(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})
	group, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Team"})

	groupRepo.AddPersonToGroup(p1.ID, group.ID, p1.ID)

	groups, err := groupRepo.ResolveGroupsForPerson(p1.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(groups) != 1 {
		t.Errorf("expected 1 group, got %d", len(groups))
	}
}

func TestResolveGroupsForPerson_WithParentGroups(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})

	// Person in child group, should also resolve to parent
	parentGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Parent", AllowSubGroups: true})
	childGroup, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Child"})

	groupRepo.AddPersonToGroup(p1.ID, childGroup.ID, p1.ID)
	groupRepo.AddSubgroup(parentGroup.ID, childGroup.ID)

	groups, _ := groupRepo.ResolveGroupsForPerson(p1.ID)

	// Should be in both child and parent (2 groups)
	if len(groups) != 2 {
		t.Errorf("expected 2 groups (child + parent), got %d: %v", len(groups), groups)
	}
}

func TestResolveGroupsForPerson_MultipleBranches(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})

	// Person in two different leaf groups, each with own parent
	parentA, _ := groupRepo.Create(models.CreateGroupRequest{Name: "ParentA", AllowSubGroups: true})
	parentB, _ := groupRepo.Create(models.CreateGroupRequest{Name: "ParentB", AllowSubGroups: true})
	childA, _ := groupRepo.Create(models.CreateGroupRequest{Name: "ChildA"})
	childB, _ := groupRepo.Create(models.CreateGroupRequest{Name: "ChildB"})

	groupRepo.AddPersonToGroup(p1.ID, childA.ID, p1.ID)
	groupRepo.AddPersonToGroup(p1.ID, childB.ID, p1.ID)
	groupRepo.AddSubgroup(parentA.ID, childA.ID)
	groupRepo.AddSubgroup(parentB.ID, childB.ID)

	groups, _ := groupRepo.ResolveGroupsForPerson(p1.ID)

	// Should be in 4 groups: childA, childB, parentA, parentB
	if len(groups) != 4 {
		t.Errorf("expected 4 groups, got %d: %v", len(groups), groups)
	}
}

func TestResolveGroupsForPerson_CircularReference(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})

	groupA, _ := groupRepo.Create(models.CreateGroupRequest{Name: "A", AllowSubGroups: true})
	groupB, _ := groupRepo.Create(models.CreateGroupRequest{Name: "B", AllowSubGroups: true})

	groupRepo.AddPersonToGroup(p1.ID, groupA.ID, p1.ID)
	groupRepo.AddSubgroup(groupA.ID, groupB.ID)
	groupRepo.AddSubgroup(groupB.ID, groupA.ID) // Circular

	groups, err := groupRepo.ResolveGroupsForPerson(p1.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 unique groups despite circular ref
	if len(groups) != 2 {
		t.Errorf("expected 2 groups despite circular ref, got %d", len(groups))
	}
}

func TestResolveGroupsForPerson_DeepNesting(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})

	// A -> B -> C -> person
	groupA, _ := groupRepo.Create(models.CreateGroupRequest{Name: "A", AllowSubGroups: true})
	groupB, _ := groupRepo.Create(models.CreateGroupRequest{Name: "B", AllowSubGroups: true})
	groupC, _ := groupRepo.Create(models.CreateGroupRequest{Name: "C"})

	groupRepo.AddPersonToGroup(p1.ID, groupC.ID, p1.ID)
	groupRepo.AddSubgroup(groupA.ID, groupB.ID)
	groupRepo.AddSubgroup(groupB.ID, groupC.ID)

	groups, _ := groupRepo.ResolveGroupsForPerson(p1.ID)

	// Should be in C, B, A (3 groups)
	if len(groups) != 3 {
		t.Errorf("expected 3 groups from deep nesting, got %d", len(groups))
	}
}

func TestResolveParticipants_EmptyGroup(t *testing.T) {
	groupRepo, _ := setupGroupTest(t)

	group, _ := groupRepo.Create(models.CreateGroupRequest{Name: "Empty"})

	participants, _ := groupRepo.ResolveParticipants(group.ID)

	if len(participants) != 0 {
		t.Errorf("expected 0 participants for empty group, got %d", len(participants))
	}
}

func TestResolveGroupsForPerson_NotInAnyGroup(t *testing.T) {
	groupRepo, personRepo := setupGroupTest(t)

	p1, _ := personRepo.Create(models.CreatePersonRequest{Name: "Alice", Email: "a@test.com", JoinedDate: "2024-01-01"})

	groups, _ := groupRepo.ResolveGroupsForPerson(p1.ID)

	if len(groups) != 0 {
		t.Errorf("expected 0 groups for person not in any group, got %d", len(groups))
	}
}
