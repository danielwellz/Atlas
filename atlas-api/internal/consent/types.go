package consent

const (
	TypeCameraFormCheck      = "camera_form_check"
	TypeProgressPhotos       = "progress_photos"
	TypeShareToCoach         = "share_to_coach"
	TypeMovementScreenCamera = "movement_screen_camera"
	TypeFormCheckLocal       = "form_check_local"
	TypeFormCheckUpload      = "form_check_upload"
	TypeProductAnalytics     = "product_analytics"
)

var allowedTypes = map[string]struct{}{
	TypeCameraFormCheck:      {},
	TypeProgressPhotos:       {},
	TypeShareToCoach:         {},
	TypeMovementScreenCamera: {},
	TypeFormCheckLocal:       {},
	TypeFormCheckUpload:      {},
	TypeProductAnalytics:     {},
}

func IsValidType(value string) bool {
	_, ok := allowedTypes[value]
	return ok
}
