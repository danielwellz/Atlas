-- +goose Up
ALTER TABLE consents
DROP CONSTRAINT IF EXISTS consents_consent_type_check;

ALTER TABLE consents
ADD CONSTRAINT consents_consent_type_check
CHECK (
    consent_type IN (
        'camera_form_check',
        'progress_photos',
        'share_to_coach',
        'movement_screen_camera',
        'form_check_local',
        'form_check_upload',
        'product_analytics'
    )
);

-- +goose Down
ALTER TABLE consents
DROP CONSTRAINT IF EXISTS consents_consent_type_check;

ALTER TABLE consents
ADD CONSTRAINT consents_consent_type_check
CHECK (consent_type IN ('camera_form_check', 'progress_photos', 'share_to_coach'));
