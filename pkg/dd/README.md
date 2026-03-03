# DataDog URL Routing

URLs matching `app.datadoghq.com` are routed to a handler based on the first path segment.

Unknown path segments fall through to `handleConnection` (connection-only check).

---

## Route Table

| URL pattern                                          | Handler            | Validation                     | Notes                                         |
|------------------------------------------------------|--------------------|--------------------------------|-----------------------------------------------|
| `app.datadoghq.com/`                                 | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/monitors`                         | `handleMonitors`   | `listMonitors`                 |                                               |
| `app.datadoghq.com/monitors/settings/…`              | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/monitors/triggered/…`             | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/monitors/quality/…`               | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/monitors/manage`                  | `handleMonitors`   | `listMonitors` (manage action) |                                               |
| `app.datadoghq.com/monitors/{id}`                    | `handleMonitors`   | `getMonitor(id)`               |                                               |
| `app.datadoghq.com/monitors/{id}/edit`               | `handleMonitors`   | `getMonitor(id)`               |                                               |
| `app.datadoghq.com/monitor/{id}`                     | `handleMonitors`   | `getMonitor(id)`               | Alias path                                    |
| `app.datadoghq.com/dashboard/{id}/…`                 | `handleDashboards` | `getDashboard(id)`             |                                               |
| `app.datadoghq.com/dashboard/lists/manual/{id}`      | `handleDashboards` | `getDashboardList(id)`         |                                               |
| `app.datadoghq.com/dashboard/lists/preset/{id}`      | `handleConnection` | Auth check only                | Preset lists not accessible via API           |
| `app.datadoghq.com/dashboard/lists`                  | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/dashboard/shared`                 | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/dashboard/reports`                | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/dash/integration/{id}/…`          | `handleConnection` | Auth check only                | Integration dashboards not accessible via API |
| `app.datadoghq.com/notebook/{id}/…`                  | `handleNotebooks`  | `getNotebook(id)`              |                                               |
| `app.datadoghq.com/notebook/custom-template/{id}/…`  | `handleNotebooks`  | `getNotebook(id)`              |                                               |
| `app.datadoghq.com/notebook/list`                    | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/notebook/reports`                 | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/notebook/template-gallery`        | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/slo/manage?sp=[{"p":{"id":"…"}}]` | `handleSLO`        | `getSLO(id)`                   | ID extracted from `sp` query param            |
| `app.datadoghq.com/slo/manage`                       | `handleSLO`        | `getSLO("")` (list check)      | No specific SLO ID                            |
| `app.datadoghq.com/sheets`                           | `handleConnection` | Auth check only                | No sheets API available                       |
| `app.datadoghq.com/sheets/{id}`                      | `handleConnection` | Auth check only                | No sheets API available                       |
| `app.datadoghq.com/ddsql/…`                          | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/check/summary`                    | `handleConnection` | Auth check only                | Not accessible via API                        |
| `app.datadoghq.com/event/overview`                   | `handleConnection` | Auth check only                | Event ID decoding not feasible                |
| `app.datadoghq.com/event/explorer`                   | `handleConnection` | Auth check only                | Event ID decoding not feasible                |
| `app.datadoghq.com/event/correlation`                | `handleConnection` | Auth check only                |                                               |
| `app.datadoghq.com/incidents/{id}`                   | `handleConnection` | Auth check only                | `v2.GetIncident` is an unstable operation     |
| `app.datadoghq.com/<unknown>`                        | `handleConnection` | Auth check only                | Falls through to connection check             |
