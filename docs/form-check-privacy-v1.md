# Form Check Privacy v1

This document describes Atlas form-check privacy controls for Step 17.

## Local-first behavior

- Form check runs on-device and is disabled until `form_check_local` consent is granted.
- Local analysis results (ROM, knee tracking, symmetry, overall score) are shown in-app without upload.
- No background uploads are performed.

## Explicit upload behavior

- Upload requires all of the following:
  - active subscription entitlement: `form_check_upload`
  - active consent: `form_check_upload`
  - explicit user tap on `Upload to Coach`
- If the user is offline, upload is blocked and local-only results remain available.

## Backend enforcement

Endpoint: `POST /api/v1/form-check/uploads`

- Requires authentication.
- Requires entitlement middleware check (`form_check_upload`).
- Requires active upload consent (`form_check_upload`).
- Persists upload metadata in `form_check_uploads`.
- Uses the `Storage` interface to normalize and validate object-storage keys before persistence.
