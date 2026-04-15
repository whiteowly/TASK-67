# Canonical API Endpoint Inventory

**Source of truth:** `internal/router/router.go`
**Extraction method:** static enumeration of every `.GET/.POST/.PATCH/.DELETE` call bound to a handler in `router.Setup`.

> ### Count reconciliation
>
> The task brief asserts "Backend has **63 API endpoints**." The router in its current state (file `internal/router/router.go`) defines **79 unique `METHOD PATH` pairs** (1 root health + 78 under `/api/v1`). All 79 are covered externally (see `docs/api-coverage-after.md`).
>
> The prompt's figure of 63 appears to reflect an earlier state of the codebase (before the admin backup/restore/archive, dashboard KPI, tickets, imports, exports, and moderation/bans routes were added). The router is the canonical source — per the task's own instruction "extract the canonical list **from router definitions**."
>
> Web (Templ) page routes under `/`, `/my/*`, `/admin` (HTML) are **out of scope** for the JSON API coverage requirement — this file lists JSON API + `/health` only. The 16 authenticated web pages + 4 admin web pages + 6 public auth/catalog pages are covered by Playwright E2E (`e2e/`) and are not included in the 79-endpoint count.

## Count summary

| Group | Count |
|---|---|
| Health | 1 |
| Auth | 3 |
| Users | 2 |
| Catalog | 4 |
| Addresses | 5 |
| Admin | 14 |
| Registrations | 6 |
| Attendance | 4 |
| Cart | 3 |
| Checkout / Buy-Now | 2 |
| Orders | 3 |
| Payments | 1 |
| Shipments | 5 |
| Posts | 4 |
| Moderation | 6 |
| Tickets | 8 |
| Imports | 5 |
| Exports | 3 |
| **TOTAL** | **79** |

## Full list (METHOD PATH, router.go line)

### Health
1. `GET /health` — router.go:30

### Auth (`/api/v1/auth`)
2. `POST /api/v1/auth/register` — router.go:59
3. `POST /api/v1/auth/login` — router.go:60
4. `POST /api/v1/auth/logout` — router.go:61

### Users (`/api/v1/users`)
5. `GET /api/v1/users/me` — router.go:67
6. `PATCH /api/v1/users/me` — router.go:68

### Catalog (`/api/v1/catalog`)
7. `GET /api/v1/catalog/sessions` — router.go:74
8. `GET /api/v1/catalog/sessions/:id` — router.go:75
9. `GET /api/v1/catalog/products` — router.go:76
10. `GET /api/v1/catalog/products/:id` — router.go:77

### Addresses (`/api/v1/addresses`)
11. `GET /api/v1/addresses` — router.go:83
12. `POST /api/v1/addresses` — router.go:84
13. `GET /api/v1/addresses/:id` — router.go:85
14. `PATCH /api/v1/addresses/:id` — router.go:86
15. `DELETE /api/v1/addresses/:id` — router.go:87

### Admin (`/api/v1/admin`)
16. `GET /api/v1/admin/config` — router.go:93
17. `PATCH /api/v1/admin/config/:key` — router.go:94
18. `GET /api/v1/admin/feature-flags` — router.go:95
19. `PATCH /api/v1/admin/feature-flags/:key` — router.go:96
20. `GET /api/v1/admin/audit-logs` — router.go:97
21. `POST /api/v1/admin/backups` — router.go:100
22. `GET /api/v1/admin/backups` — router.go:101
23. `POST /api/v1/admin/restore` — router.go:102
24. `GET /api/v1/admin/archives` — router.go:103
25. `POST /api/v1/admin/archives` — router.go:104
26. `POST /api/v1/admin/refunds/:id/reconcile` — router.go:107
27. `GET /api/v1/admin/kpis` — router.go:128
28. `GET /api/v1/admin/jobs` — router.go:129
29. `POST /api/v1/admin/registrations/override` — router.go:132

### Registrations (`/api/v1/registrations`)
30. `POST /api/v1/registrations` — router.go:138
31. `GET /api/v1/registrations` — router.go:139
32. `GET /api/v1/registrations/:id` — router.go:140
33. `POST /api/v1/registrations/:id/cancel` — router.go:141
34. `POST /api/v1/registrations/:id/approve` — router.go:142
35. `POST /api/v1/registrations/:id/reject` — router.go:143

### Attendance (`/api/v1/attendance`)
36. `POST /api/v1/attendance/checkin` — router.go:149
37. `POST /api/v1/attendance/leave` — router.go:150
38. `POST /api/v1/attendance/leave/:id/return` — router.go:151
39. `GET /api/v1/attendance/exceptions` — router.go:152

### Cart (`/api/v1/cart`)
40. `GET /api/v1/cart` — router.go:158
41. `POST /api/v1/cart/items` — router.go:159
42. `DELETE /api/v1/cart/items/:id` — router.go:160

### Checkout / Buy-Now
43. `POST /api/v1/checkout` — router.go:164
44. `POST /api/v1/buy-now` — router.go:165

### Orders (`/api/v1/orders`)
45. `GET /api/v1/orders` — router.go:170
46. `GET /api/v1/orders/:id` — router.go:171
47. `POST /api/v1/orders/:id/pay` — router.go:172

### Payments (`/api/v1/payments`)
48. `POST /api/v1/payments/callback` — router.go:178

### Shipments (`/api/v1/shipments`)
49. `POST /api/v1/shipments` — router.go:184
50. `GET /api/v1/shipments` — router.go:185
51. `PATCH /api/v1/shipments/:id/status` — router.go:186
52. `POST /api/v1/shipments/:id/pod` — router.go:187
53. `POST /api/v1/shipments/:id/exception` — router.go:188

### Posts (`/api/v1/posts`)
54. `GET /api/v1/posts` — router.go:194
55. `GET /api/v1/posts/:id` — router.go:195
56. `POST /api/v1/posts` — router.go:196
57. `POST /api/v1/posts/:id/report` — router.go:197

### Moderation (`/api/v1/moderation`)
58. `GET /api/v1/moderation/reports` — router.go:203
59. `GET /api/v1/moderation/cases` — router.go:204
60. `GET /api/v1/moderation/cases/:id` — router.go:205
61. `POST /api/v1/moderation/cases/:id/action` — router.go:206
62. `POST /api/v1/moderation/bans` — router.go:207
63. `POST /api/v1/moderation/bans/:id/revoke` — router.go:208

### Tickets (`/api/v1/tickets`)
64. `POST /api/v1/tickets` — router.go:214
65. `GET /api/v1/tickets` — router.go:215
66. `GET /api/v1/tickets/:id` — router.go:216
67. `PATCH /api/v1/tickets/:id/status` — router.go:217
68. `POST /api/v1/tickets/:id/assign` — router.go:218
69. `POST /api/v1/tickets/:id/comments` — router.go:219
70. `POST /api/v1/tickets/:id/resolve` — router.go:220
71. `POST /api/v1/tickets/:id/close` — router.go:221

### Imports (`/api/v1/imports`)
72. `POST /api/v1/imports` — router.go:227
73. `GET /api/v1/imports` — router.go:228
74. `GET /api/v1/imports/:id` — router.go:229
75. `POST /api/v1/imports/:id/validate` — router.go:230
76. `POST /api/v1/imports/:id/apply` — router.go:231

### Exports (`/api/v1/exports`)
77. `POST /api/v1/exports` — router.go:237
78. `GET /api/v1/exports` — router.go:238
79. `GET /api/v1/exports/:id/download` — router.go:239
