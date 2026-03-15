# People Service API Documentation

Base URL: `http://localhost:8080`

All endpoints require no authentication (AuthType: none). Group management endpoints that modify state require the `X-Person-Id` header to identify the requestor.

---

## Table of Contents

- [Health Check](#health-check)
- [People API](#people-api)
  - [Create Person](#create-person)
  - [Search People](#search-people)
  - [Update Person](#update-person)
  - [Deactivate Person](#deactivate-person)
  - [Reactivate Person](#reactivate-person)
- [Groups API](#groups-api)
  - [Create Group](#create-group)
  - [Update Group](#update-group)
  - [Add Member to Group](#add-member-to-group)
  - [Remove Member from Group](#remove-member-from-group)
  - [Bulk Manage Members](#bulk-manage-members)
  - [List Direct Members](#list-direct-members)
  - [Check Membership](#check-membership)
  - [Add Subgroup to Group](#add-subgroup-to-group)
  - [Remove Subgroup from Group](#remove-subgroup-from-group)
- [Search API](#search-api)
  - [Unified Search](#unified-search)

---

## Health Check

### GET /health

Check if the service is running.

```bash
curl -s http://localhost:8080/health
```

**Response (200 OK):**

```json
{"status": "healthy"}
```

---

## People API

### Create Person

**POST /people**

Create a new person.

```bash
curl -s -X POST http://localhost:8080/people \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Alice Smith",
    "email": "alice.smith@example.com",
    "joined_date": "2024-01-15"
  }'
```

**Response (201 Created):**

```json
{
  "id": 1,
  "name": "Alice Smith",
  "email": "alice.smith@example.com",
  "is_active": 1,
  "joined_date": "2024-01-15",
  "deactived_at": null,
  "activated_at": null
}
```

**Validation Errors (400 Bad Request):**

- Missing required fields (`name`, `email`, `joined_date`)
- Invalid JSON body

```bash
# Missing fields example
curl -s -X POST http://localhost:8080/people \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice"}'
```

---

### Search People

**GET /people?q=<query>**

Search people by substring match on name or email (case-insensitive). Omit `q` to list all.

```bash
# Search by name/email substring
curl -s "http://localhost:8080/people?q=alice"

# List all people
curl -s http://localhost:8080/people
```

**Response (200 OK):**

```json
[
  {
    "id": 1,
    "name": "Alice Smith",
    "email": "alice.smith@example.com",
    "is_active": 1,
    "joined_date": "2024-01-15",
    "deactived_at": null,
    "activated_at": null
  }
]
```

---

### Update Person

**PUT /people/{id}**

Partially update a person's details. Only include fields to change.

```bash
curl -s -X PUT http://localhost:8080/people/1 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Alice Johnson",
    "email": "alice.johnson@example.com"
  }'
```

**Response (200 OK):**

```json
{
  "id": 1,
  "name": "Alice Johnson",
  "email": "alice.johnson@example.com",
  "is_active": 1,
  "joined_date": "2024-01-15",
  "deactived_at": null,
  "activated_at": null
}
```

**Errors:**

- `404 Not Found` – Person does not exist
- `400 Bad Request` – Invalid ID or JSON body

```bash
# Update non-existing person
curl -s -X PUT http://localhost:8080/people/999 \
  -H "Content-Type: application/json" \
  -d '{"name": "Test"}'
```

---

### Deactivate Person

**POST /people/{id}/deactivate**

Sets `is_active=0` and `deactived_at=now()`.

```bash
curl -s -X POST http://localhost:8080/people/1/deactivate
```

**Response (200 OK):**

```json
{
  "id": 1,
  "name": "Alice Johnson",
  "email": "alice.johnson@example.com",
  "is_active": 0,
  "joined_date": "2024-01-15",
  "deactived_at": "2024-01-20 10:30:00",
  "activated_at": null
}
```

**Errors:**

- `404 Not Found` – Person does not exist
- `400 Bad Request` – Invalid ID

```bash
# Deactivate non-existing
curl -s -X POST http://localhost:8080/people/999/deactivate
```

---

### Reactivate Person

**POST /people/{id}/reactivate**

Sets `is_active=1` and `activated_at=now()`.

```bash
curl -s -X POST http://localhost:8080/people/1/reactivate
```

**Response (200 OK):**

```json
{
  "id": 1,
  "name": "Alice Johnson",
  "email": "alice.johnson@example.com",
  "is_active": 1,
  "joined_date": "2024-01-15",
  "deactived_at": "2024-01-20 10:30:00",
  "activated_at": "2024-01-21 09:00:00"
}
```

**Errors:**

- `404 Not Found` – Person does not exist
- `400 Bad Request` – Invalid ID

---

## Groups API

### Create Group

**POST /groups**

Create a new group. Requires `X-Person-Id` header identifying the creator.

- If `admin_group_id` is omitted, an admin group named `<GroupName>-Admins` is auto-created.
- The requestor is added to both the group and the admin group.

```bash
curl -s -X POST http://localhost:8080/groups \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{
    "name": "Developers",
    "description": "Software development team",
    "members_visible": false,
    "allow_self_add": false,
    "allow_sub_groups": true
  }'
```

**Response (201 Created):**

```json
{
  "id": 1,
  "name": "Developers",
  "description": "Software development team",
  "members_visible": 0,
  "allow_self_add": 0,
  "allow_sub_groups": 1,
  "admin_group_id": 2
}
```

**Auto-created admin group:** `Developers-Admins` (group id 2).

**Errors:**

- `400 Bad Request` – Missing/invalid `X-Person-Id` header, missing `name`
- `404 Not Found` – Requestor person does not exist
- `500 Internal Server Error` – Database failure

```bash
# Missing header
curl -s -X POST http://localhost:8080/groups \
  -H "Content-Type: application/json" \
  -d '{"name": "Test"}'

# Invalid JSON
curl -s -X POST http://localhost:8080/groups \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d 'not-json'
```

---

### Update Group

**PUT /groups/{id}**

Update group options. Only admin group members (directly or via subgroups) may update.

```bash
curl -s -X PUT http://localhost:8080/groups/1 \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{
    "description": "Updated description",
    "members_visible": true
  }'
```

**Response (200 OK):**

```json
{
  "id": 1,
  "name": "Developers",
  "description": "Updated description",
  "members_visible": 1,
  "allow_self_add": 0,
  "allow_sub_groups": 1,
  "admin_group_id": 2
}
```

**Partial updates supported** – only include fields to change.

**Errors:**

- `400 Bad Request` – Invalid ID, invalid JSON, missing `X-Person-Id`
- `403 Forbidden` – Requestor is not in admin group
- `404 Not Found` – Group does not exist

```bash
# Non-admin tries to update
curl -s -X PUT http://localhost:8080/groups/1 \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 999" \
  -d '{"description": "Hacked"}'

# Group not found
curl -s -X PUT http://localhost:8080/groups/999 \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"description": "Test"}'
```

---

### Add Member to Group

**POST /groups/{id}/members**

Add a person as a direct member. Only admin group members may add.

```bash
curl -s -X POST http://localhost:8080/groups/1/members \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"person_id": 3}'
```

**Response (201 Created):**

```json
{"status": "added"}
```

**Errors:**

- `400 Bad Request` – Missing/invalid `X-Person-Id`, missing `person_id`, invalid ID
- `403 Forbidden` – Requestor not in admin group
- `404 Not Found` – Group or target person not found

```bash
# Non-admin tries to add
curl -s -X POST http://localhost:8080/groups/1/members \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 999" \
  -d '{"person_id": 3}'

# Person not found
curl -s -X POST http://localhost:8080/groups/1/members \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"person_id": 999}'
```

---

### Remove Member from Group

**DELETE /groups/{id}/members/{personId}**

Removes a person from the group. Only admin group members may remove others.

**Restrictions:**
- Requestor cannot remove themselves
- Cannot remove another admin (even if you are an admin)

```bash
curl -s -X DELETE http://localhost:8080/groups/1/members/3 \
  -H "X-Person-Id: 1"
```

**Response (200 OK):**

```json
{"status": "removed"}
```

**Errors:**

- `400 Bad Request` – Missing/invalid `X-Person-Id`, invalid ID
- `403 Forbidden` – Requestor not admin, trying to remove self, or trying to remove another admin
- `404 Not Found` – Group or target person not found

```bash
# Non-admin tries to remove
curl -s -X DELETE http://localhost:8080/groups/1/members/3 \
  -H "X-Person-Id: 999"
# -> 403: {"error": "requestor is not admin"}

# Admin tries to remove themselves
curl -s -X DELETE http://localhost:8080/groups/1/members/1 \
  -H "X-Person-Id: 1"
# -> 403: {"error": "cannot remove yourself from the group"}

# Admin tries to remove another admin
curl -s -X DELETE http://localhost:8080/groups/1/members/2 \
  -H "X-Person-Id: 1"
# -> 403: {"error": "cannot remove another admin from the group"}

# Person not found
curl -s -X DELETE http://localhost:8080/groups/1/members/999 \
  -H "X-Person-Id: 1"
# -> 404: {"error": "person not found"}
```

---

### Bulk Manage Members

**POST /groups/{id}/bulk-members**

Bulk add or remove multiple members from a group in a single atomic operation.

- **Atomic**: All operations succeed or none are applied (transaction-based)
- **Idempotent**: Adding an existing member or removing a non-member returns success
- **Maximum 100 person_ids per request**
- Requires `X-Person-Id` header and admin access

**Request Body:**

```json
{
  "person_ids": [2, 3, 4],
  "action": "add"  // or "remove"
}
```

**Add Example:**

```bash
curl -s -X POST http://localhost:8080/groups/1/bulk-members \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"person_ids": [2, 3, 4], "action": "add"}'
```

**Remove Example:**

```bash
curl -s -X POST http://localhost:8080/groups/1/bulk-members \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"person_ids": [2, 3], "action": "remove"}'
```

**Response (200 OK):**

```json
{
  "total_requested": 3,
  "total_success": 3,
  "total_failed": 0,
  "results": [
    {"person_id": 2, "success": true},
    {"person_id": 3, "success": true},
    {"person_id": 4, "success": true}
  ]
}
```

**Partial Failure Response:**

```json
{
  "total_requested": 3,
  "total_success": 2,
  "total_failed": 1,
  "results": [
    {"person_id": 2, "success": true},
    {"person_id": 3, "success": false, "error": "failed to add member"},
    {"person_id": 4, "success": true}
  ]
}
```

**Errors:**

- `400 Bad Request` – Missing/invalid `X-Person-Id`, invalid JSON, empty `person_ids`, `action` not "add" or "remove", more than 100 person_ids
- `403 Forbidden` – Requestor not admin
- `404 Not Found` – Group not found or any person_id not found

```bash
# Empty array
curl -s -X POST http://localhost:8080/groups/1/bulk-members \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"person_ids": [], "action": "add"}'
# -> 400: {"error": "person_ids array is required and cannot be empty"}

# Too many IDs
curl -s -X POST http://localhost:8080/groups/1/bulk-members \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"person_ids": [1,2,3,...101], "action": "add"}'
# -> 400: {"error": "maximum 100 person_ids allowed per request"}

# Invalid action
curl -s -X POST http://localhost:8080/groups/1/bulk-members \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"person_ids": [2], "action": "delete"}'
# -> 400: {"error": "action must be 'add' or 'remove'"}

# Non-admin tries bulk operation
curl -s -X POST http://localhost:8080/groups/1/bulk-members \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 999" \
  -d '{"person_ids": [2, 3], "action": "add"}'
# -> 403: {"error": "requestor is not admin"}
```

---

### List Direct Members

**GET /groups/{id}/members/direct**

Returns the direct (non-resolved) list of person IDs and subgroup IDs.

```bash
curl -s http://localhost:8080/groups/1/members/direct
```

**Response (200 OK):**

```json
{
  "people": [1, 3],
  "subgroups": [5]
}
```

**Note:** This does NOT resolve nested subgroups or parent groups. Use `Check Membership` for resolved membership.

**Errors:**

- `400 Bad Request` – Invalid group ID
- `404 Not Found` – Group does not exist

```bash
# Group not found
curl -s http://localhost:8080/groups/999/members/direct
```

---

### Check Membership

**GET /groups/{id}/members/{personId}/check**

Returns whether a person is a member of the group (resolved through subgroups).

```bash
curl -s http://localhost:8080/groups/1/members/3/check
```

**Response (200 OK):**

```json
{"is_member": true}
```

```bash
# Not a member
curl -s http://localhost:8080/groups/1/members/999/check
```

**Response:**

```json
{"is_member": false}
```

**Errors:**

- `400 Bad Request` – Invalid group ID or person ID
- `404 Not Found` – Group does not exist

```bash
# Invalid person ID
curl -s http://localhost:8080/groups/1/members/abc/check
```

---

### Add Subgroup to Group

**POST /groups/{id}/subgroups**

Adds another group as a subgroup of the specified group. Requires `X-Person-Id` header and admin access.

- Parent group must have `allow_sub_groups: true`
- Requestor must be a member of the parent group's admin group (directly or via subgroups)
- Cannot add a group as subgroup of itself
- Cannot create circular references

```bash
curl -s -X POST http://localhost:8080/groups/1/subgroups \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"subgroup_id": 5}'
```

**Response (201 Created):**

```json
{
  "status": "added",
  "parent_id": 1,
  "subgroup_id": 5
}
```

**Errors:**

- `400 Bad Request` – Missing/invalid `X-Person-Id`, missing `subgroup_id`, invalid ID, self-reference, circular reference
- `403 Forbidden` – Subgroups not allowed on parent group, or requestor not admin
- `404 Not Found` – Parent group or subgroup not found

```bash
# Subgroups not allowed
curl -s -X POST http://localhost:8080/groups/1/subgroups \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"subgroup_id": 5}'
# -> 403: {"error": "subgroups not allowed for this group"}

# Non-admin tries to add
curl -s -X POST http://localhost:8080/groups/1/subgroups \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 999" \
  -d '{"subgroup_id": 5}'
# -> 403: {"error": "requestor is not admin"}
```

---

### Remove Subgroup from Group

**DELETE /groups/{id}/subgroups/{subgroupId}**

Removes a subgroup relationship. Requires `X-Person-Id` header and admin access to parent group.

```bash
curl -s -X DELETE http://localhost:8080/groups/1/subgroups/5 \
  -H "X-Person-Id: 1"
```

**Response (200 OK):**

```json
{"status": "removed"}
```

**Errors:**

- `400 Bad Request` – Missing/invalid `X-Person-Id`, invalid ID
- `403 Forbidden` – Requestor not admin
- `404 Not Found` – Parent group not found

```bash
# Non-admin tries to remove
curl -s -X DELETE http://localhost:8080/groups/1/subgroups/5 \
  -H "X-Person-Id: 999"
# -> 403: {"error": "requestor is not admin"}
```

---

## Search API

### Unified Search

**GET /search?q={query}**

Performs full-text search across both people and groups, returning combined results.

- **FTS5 powered**: When `USE_FTS=true` in .env, uses SQLite FTS5 for faster, more accurate results
- **Automatic fallback**: Falls back to LIKE queries if FTS is unavailable
- **Ranked results**: FTS results are ranked with exact matches prioritized
- **No auth required**

**Query Parameters:**

| Parameter | Required | Description |
|-----------|----------|-------------|
| q | Yes | Search query string (max 200 chars) |

**Example:**

```bash
curl -s "http://localhost:8080/search?q=alice"
```

**Response (200 OK):**

```json
{
  "query": "alice",
  "fts": true,
  "people": [
    {
      "id": 1,
      "name": "Alice Smith",
      "email": "alice.smith@example.com",
      "is_active": 1,
      "joined_date": "2024-01-15"
    }
  ],
  "groups": [
    {
      "id": 5,
      "name": "Alice's Team",
      "description": "Team led by Alice",
      "allow_sub_groups": 0
    }
  ],
  "total": 2
}
```

**Advanced FTS Queries:**

```bash
# Phrase search (exact phrase)
curl -s "http://localhost:8080/search?q=\"john doe\""

# Prefix search (starts with)
curl -s "http://localhost:8080/search?q=al*"

# Boolean AND
curl -s "http://localhost:8080/search?q=alice AND smith"

# Boolean OR
curl -s "http://localhost:8080/search?q=alice OR bob"

# NEAR (words near each other)
curl -s "http://localhost:8080/search?q=NEAR(alice team)"

# Column-specific search (FTS only)
# Search only in email column
curl -s "http://localhost:8080/search?q=email:test"

# Exclude terms
curl -s "http://localhost:8080/search?q=alice NOT test"
```

**With FTS disabled (USE_FTS=false or not set):**

```bash
# Uses LIKE queries - simple substring matching
curl -s "http://localhost:8080/search?q=ali"
# Matches: "Alice", "Alice Smith", "ali@test.com", etc.
```

**Errors:**

- `400 Bad Request` – Missing `q` parameter

---

## Common Error Responses

| Status | Meaning | Example |
|--------|---------|---------|
| `400` | Bad Request – invalid input | Missing required field, invalid JSON, invalid ID |
| `403` | Forbidden – not authorized | Not in admin group for group operations |
| `404` | Not Found – resource missing | Person/group not found |
| `500` | Internal Server Error – server issue | Database failure |

---

## Example Workflow

```bash
# 1. Create a person
curl -s -X POST http://localhost:8080/people \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@test.com","joined_date":"2024-01-01"}'
# -> {"id":1,...}

# 2. Create a group (Alice is auto-added to group + admin group)
curl -s -X POST http://localhost:8080/groups \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"name":"Engineering","description":"Eng team","allow_sub_groups":true}'
# -> {"id":1,"admin_group_id":2,...}

# 3. Create another person
curl -s -X POST http://localhost:8080/people \
  -H "Content-Type: application/json" \
  -d '{"name":"Bob","email":"bob@test.com","joined_date":"2024-01-01"}'
# -> {"id":2,...}

# 4. Add Bob to Engineering (Alice is admin)
curl -s -X POST http://localhost:8080/groups/1/members \
  -H "Content-Type: application/json" \
  -H "X-Person-Id: 1" \
  -d '{"person_id":2}'
# -> {"status":"added"}

# 5. Check if Bob is in Engineering
curl -s http://localhost:8080/groups/1/members/2/check
# -> {"is_member":true}

# 6. List direct members of Engineering
curl -s http://localhost:8080/groups/1/members/direct
# -> {"people":[1,2],"subgroups":[]}

# 7. Deactivate Alice
curl -s -X POST http://localhost:8080/people/1/deactivate
# -> {"id":1,"is_active":0,...}
```
