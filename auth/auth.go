package auth

type Auth interface {
	Authenticate(pubKey, appName string) error
}
