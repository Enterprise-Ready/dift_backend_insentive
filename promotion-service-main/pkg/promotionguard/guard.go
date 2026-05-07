package promotionguard

import "strings"

func RequireDriverID(driverID string) bool { return strings.TrimSpace(driverID) != "" }
func NormalizeID(id string) string         { return strings.TrimSpace(id) }
