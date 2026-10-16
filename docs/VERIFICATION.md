# Verification Principles and Known Limitations

Routine verification covers only the current change and uses isolated synthetic data; see [DEPLOYMENT.md](DEPLOYMENT.md) for the specific release and rollback procedures. Historical regression suites and per-run test-case counts have been retired; the original records remain available in Git history.

Image processing depends on libvips. When a change affects images or backups, verify the relevant input formats, output readability, and database/photo consistency. A real device is required to confirm phone-camera behavior, HEIC export from the photo library, and the soft keyboard; a desktop mobile viewport cannot substitute for real-device results. Standard photos have a maximum long edge of 2560 px; original camera files are not retained.

Production uses HTTPS and centralized device authentication. Daily backups are same-disk snapshots; one manual off-machine archive exists, but there is no automated off-machine synchronization. Read `current-commit` for the actual deployed version and `docs/SOURCE` for the documentation source. Check disk and service state live rather than relying on an old PID or version number.
