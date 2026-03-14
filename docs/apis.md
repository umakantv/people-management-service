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
  - [List Direct Members](#list-direct-members)
  - [Check Membership](#check-membership)

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
