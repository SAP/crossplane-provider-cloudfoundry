package clients

import "github.com/google/uuid"

// IsValidGUID checks if the given string is a valid UUID.
func IsValidGUID(guid string) bool {
	_, err := uuid.Parse(guid)
	return err == nil
}
