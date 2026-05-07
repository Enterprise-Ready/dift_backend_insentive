package rewardguard

import (
	"errors"
	_ "github.com/PlatformCore/libpackage/validation"
	"strings"
)

func ValidateUserID(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return errors.New("user_id_required")
	}
	return nil
}
func ValidatePositivePoint(point int64) error {
	if point <= 0 {
		return errors.New("point_must_be_positive")
	}
	return nil
}
