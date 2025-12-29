package security

type AccessTokenVerifier interface {
	VerifyAccessToken(token string) (TokenClaims, error)
}
