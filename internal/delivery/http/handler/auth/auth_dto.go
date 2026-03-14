package auth

type otpSendRequest struct {
	Email string `json:"email" validate:"required,email" case:"lower"`
}

type otpVerifyRequest struct {
	Email string `json:"email" validate:"required,email" case:"lower"`
	Code  string `json:"code"  validate:"required" case:"upper"`
}

type oauthCallbackQuery struct {
	Code  string `query:"code"  validate:"required"`
	State string `query:"state" validate:"required"`
}
