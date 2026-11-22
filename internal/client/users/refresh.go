package users

import "context"

// RefreshService расширяет Users API для браузерной ротации без access JWT.
type RefreshService interface {
	RefreshSession(context.Context, string) (*AuthPayload, error)
	RevokeRefreshSession(context.Context, string) error
}

func (c *Client) RefreshSession(ctx context.Context, token string) (*AuthPayload, error) {
	var data struct {
		RefreshSession *AuthPayload `json:"refreshSession"`
	}
	err := c.call(ctx, "RefreshSession", `mutation RefreshSession($refreshToken:String!){refreshSession(refreshToken:$refreshToken){token expiresAt refreshToken refreshExpiresAt user{`+userFields+`}}}`, map[string]any{"refreshToken": token}, false, &data)
	if err != nil {
		return nil, err
	}
	if data.RefreshSession == nil {
		return nil, &Error{Code: codeBadGateway, Message: "empty refresh response"}
	}

	return data.RefreshSession, nil
}

func (c *Client) RevokeRefreshSession(ctx context.Context, token string) error {
	var data struct {
		Revoked bool `json:"revokeRefreshSession"`
	}
	if err := c.call(ctx, "RevokeRefreshSession", `mutation RevokeRefreshSession($refreshToken:String!){revokeRefreshSession(refreshToken:$refreshToken)}`, map[string]any{"refreshToken": token}, false, &data); err != nil {
		return err
	}
	if !data.Revoked {
		return &Error{Code: codeBadGateway, Message: "refresh revocation unavailable"}
	}

	return nil
}
