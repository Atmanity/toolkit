package api

import (
	"context"

	"github.com/Atmanity/toolkit/cli/internal/avatarapi"
)

// The account reads, as the API answers them (camelCase, microcredits).
type (
	Balance     = avatarapi.AccountBalance
	Account     = avatarapi.Account
	CurrentUser = avatarapi.CurrentCaller
)

// Balance is the account's credits: what a session can spend now, the
// buckets, what live sessions hold and what expires when.
func (c *Client) Balance(ctx context.Context) (*Balance, error) {
	r, err := c.gen.GetAccountBalanceWithResponse(ctx)
	if r == nil {
		return nil, err
	}
	return decode[Balance](r.HTTPResponse, r.Body, err)
}

// Account is the plan, limits, prices and capture quota.
func (c *Client) Account(ctx context.Context) (*Account, error) {
	r, err := c.gen.GetAccountWithResponse(ctx)
	if r == nil {
		return nil, err
	}
	return decode[Account](r.HTTPResponse, r.Body, err)
}

// CurrentKey is the key making the call: name, scopes, spend cap.
func (c *Client) CurrentKey(ctx context.Context) (*CurrentUser, error) {
	r, err := c.gen.GetCurrentApiKeyWithResponse(ctx)
	if r == nil {
		return nil, err
	}
	return decode[CurrentUser](r.HTTPResponse, r.Body, err)
}
