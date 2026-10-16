# Project Documentation

| Topic | Document |
| --- | --- |
| Architecture, data, and image lifecycle | [ARCHITECTURE.md](ARCHITECTURE.md) |
| Finished-work module design and rollout steps | [WORKS.md](WORKS.md) |
| Custom choice controls and top-bar navigation | [CONTROLS.md](CONTROLS.md) |
| HTTP endpoints and errors | [API.md](API.md) |
| Installation, backup, recovery, and diagnostics | [OPERATIONS.md](OPERATIONS.md) |
| Local-machine release and rollback | [DEPLOYMENT.md](DEPLOYMENT.md) |
| Verification evidence and unverified items | [VERIFICATION.md](VERIFICATION.md) |
| Reviewed design baseline | [TECHNICAL_PLAN.md](TECHNICAL_PLAN.md) |

The shared server-maintenance entry point is `server-operations`; its server copy is at `/opt/server-context`. The shared fabric and finished-work libraries use centralized device authentication, and authorized devices can view, edit, delete, and export.

Follow the [local-machine release guide](DEPLOYMENT.md) for routine releases and verification. GitHub is a source-code backup only.
