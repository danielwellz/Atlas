# Atlas Privacy and Compliance Readiness Checklist

This checklist is the staging-to-production baseline for Atlas API and mobile clients.

## 1. Data inventory

| Data domain | Examples | Primary system | Sensitivity | Minimum control baseline |
| --- | --- | --- | --- | --- |
| Identity and auth | email, password hash, session tokens | Postgres (`users`, `sessions`) | Personal data | bcrypt password hashing, TLS in transit, restricted DB access |
| Consent records | consent type, granted/revoked timestamps, legal text version | Postgres (`consents`) | Regulatory evidence | immutable audit trail, UTC timestamps, change history retained |
| Fitness and health-adjacent profile | goals, workout logs, readiness, nutrition logs, weight history | Postgres domain tables | High sensitivity (health-adjacent) | least-privilege access, encrypted backups, retention policy |
| Form-check uploads metadata | media object key, upload context, coach review metadata | Postgres + object storage | Potential biometric/health data | explicit opt-in consent + entitlement, encrypted object storage, short-lived object URLs |
| Product analytics | event name, anonymous/user identifiers, app/device context | Postgres analytics tables | Behavioral telemetry | privacy-safe schema, consent gate, documented purpose limitation |
| Billing/subscription metadata | app store platform, receipt/transaction identifiers | Postgres billing tables | Financial + account linkage | access control, non-production redaction in logs, audit access |

## 2. Consent flows

### Implemented controls

- `GET /api/v1/consents`: list active/revoked consent state for the signed-in user.
- `POST /api/v1/consents/grant`: explicit grant flow.
- `POST /api/v1/consents/revoke`: explicit revoke flow.
- `POST /api/v1/form-check/uploads`: blocks unless entitlement and upload consent are active.
- `POST /api/v1/events`: enforces product analytics consent (`product_analytics`) for authenticated users; anonymous clients must send `consentGranted=true`.

### Required hardening before production

- [ ] Store consent policy version and disclosure copy hash with each grant/revoke event.
- [ ] Add consent revocation propagation SLA (for example: downstream processors honor revoke within 24h).
- [ ] Add integration tests that verify all sensitive endpoints fail closed when consent is revoked.

## 3. Deletion and export endpoints (DSAR readiness)

### Current status

- Account and domain data APIs exist (`/api/v1/me`, workouts, nutrition, etc.), but dedicated DSAR workflows are not yet exposed.

### Required controls

- [ ] Add user data export endpoint(s), for example `POST /api/v1/privacy/export` with async job status endpoint.
- [ ] Add account/data deletion endpoint(s), for example `DELETE /api/v1/privacy/account` with async deletion job.
- [ ] Define deletion scope matrix (hard-delete vs soft-delete vs legal hold) for each table/object-store path.
- [ ] Define and document DSAR SLA (for example 30 days) and support escalation path.

## 4. Incident response basics

- [ ] Publish a security incident runbook (roles, severity, escalation contacts, evidence checklist).
- [ ] Ensure centralized logs + OTel traces are retained and access-controlled for forensic review.
- [ ] Define breach-notification decision tree with legal review gate (GDPR 72-hour window where applicable).
- [ ] Run at least one tabletop exercise per quarter and capture action items.

## 5. Apple privacy disclosures and privacy manifests workflow

Current repo state includes `atlas-mobile/ios/AtlasMobile/PrivacyInfo.xcprivacy`.

Required workflow:

- [ ] Keep iOS privacy manifest aligned with runtime data collection and required-reason APIs.
- [ ] Review third-party SDK manifests during dependency upgrades; reconcile additions before release.
- [ ] Add release checklist step: compare App Store Connect privacy answers against `PrivacyInfo.xcprivacy` and backend behavior.
- [ ] For each new mobile capability, require privacy review sign-off before merge.

## 6. FTC HBNR and GDPR special-category considerations

Atlas processes fitness and potentially biometric-adjacent data. Treat this as high-risk personal data by default.

- [ ] Conduct and document a Data Protection Impact Assessment (DPIA) for form-check uploads and health-adjacent analytics.
- [ ] Minimize collection and retention of health-related data fields to what is strictly needed for product function.
- [ ] Treat consumer-facing promises as enforceable commitments (FTC Act Section 5 / HBNR risk); keep disclosures specific and accurate.
- [ ] Implement breach response pathways that can satisfy both HBNR-style consumer notice obligations and GDPR supervisory authority timelines when jurisdictionally applicable.
- [ ] Route all launches that introduce new health/biometric processing through legal/privacy review before enabling by default.

## 7. Release gate

Before production launch, all unchecked items above must be either:

1. Completed, or
2. Explicitly risk-accepted in writing by product + security + legal owners.
