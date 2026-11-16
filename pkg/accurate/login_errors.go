package accurate

import "fmt"

// LoginStage identifies a fixed step of the web authentication protocol.
type LoginStage string

const (
	LoginStageInitialPage     LoginStage = "initial_page"
	LoginStagePreLogin        LoginStage = "pre_login"
	LoginStageMFAVerification LoginStage = "mfa_verification"
	LoginStageAuthentication  LoginStage = "authentication"
)

// LoginError reports where authentication failed without exposing upstream
// bodies, credentials, cookies, permits, or request/redirect URLs.
// Err is retained only for errors.Is/errors.As; Error never renders it.
type LoginError struct {
	Stage      LoginStage
	StatusCode int
	Err        error
}

func (e *LoginError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("Accurate login failed at %s (upstream HTTP %d)", e.Stage, e.StatusCode)
	}
	return fmt.Sprintf("Accurate login failed at %s", e.Stage)
}

func (e *LoginError) Unwrap() error { return e.Err }
