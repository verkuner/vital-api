package auth

import (
	"github.com/org/vital-api/authprovider"
)

var secrets struct {
	SupabaseURL            string
	SupabaseAnonKey        string
	SupabaseServiceRoleKey string
}

// newAuthProvider creates the AuthProvider based on configured secrets.
// Currently defaults to Supabase for local/dev.
func newAuthProvider() authprovider.AuthProvider {
	return authprovider.NewSupabaseProvider(
		secrets.SupabaseURL,
		secrets.SupabaseAnonKey,
		secrets.SupabaseServiceRoleKey,
	)
}
