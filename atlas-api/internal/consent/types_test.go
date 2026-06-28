package consent

import "testing"

func TestIsValidType(t *testing.T) {
	t.Parallel()

	validTypes := []string{
		TypeCameraFormCheck,
		TypeProgressPhotos,
		TypeShareToCoach,
		TypeMovementScreenCamera,
		TypeFormCheckLocal,
		TypeFormCheckUpload,
		TypeProductAnalytics,
	}

	for _, validType := range validTypes {
		validType := validType
		t.Run(validType, func(t *testing.T) {
			t.Parallel()
			if !IsValidType(validType) {
				t.Fatalf("expected consent type %q to be valid", validType)
			}
		})
	}

	if IsValidType("invalid-consent") {
		t.Fatal("expected invalid consent type to be rejected")
	}
}
