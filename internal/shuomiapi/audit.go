package shuomiapi

import (
	"context"
	"net/url"
	"strconv"
	"time"
)

type AuditQuery struct {
	Cursor  string `json:"cursor"`
	Actor   string `json:"actor"`
	Action  string `json:"action"`
	Outcome string `json:"outcome"`
	From    string `json:"from"`
	To      string `json:"to"`
}
type AuditItem struct {
	EventID          string     `json:"eventId"`
	OperationID      string     `json:"operationId"`
	ActorSubject     string     `json:"actorSubject"`
	MemberRef        string     `json:"memberRef"`
	Action           string     `json:"action"`
	ServerTime       time.Time  `json:"serverTime"`
	ClientObservedAt *time.Time `json:"clientObservedAt"`
	Outcome          string     `json:"outcome"`
	ReasonCode       string     `json:"reasonCode"`
	ProfileRef       string     `json:"profileRef"`
	BindingRef       string     `json:"bindingRef"`
	FactSource       string     `json:"factSource"`
}
type AuditPage struct {
	SchemaVersion  string      `json:"schemaVersion"`
	UserID         string      `json:"userId"`
	OrganizationID string      `json:"organizationId"`
	StoreID        string      `json:"storeId"`
	Items          []AuditItem `json:"items"`
	NextCursor     *string     `json:"nextCursor"`
	Coverage       []string    `json:"coverage"`
	CanFilterActor bool        `json:"canFilterActor"`
}

func (c *Client) Audit(ctx context.Context, g uint64, org, store string, q AuditQuery) (AuditPage, error) {
	if !canonicalID(store) {
		return AuditPage{}, ErrContext
	}
	values := url.Values{"limit": {strconv.Itoa(20)}}
	for k, v := range map[string]string{"cursor": q.Cursor, "actor": q.Actor, "action": q.Action, "outcome": q.Outcome, "from": q.From, "to": q.To} {
		if len(v) > 4096 {
			return AuditPage{}, ErrContext
		}
		if v != "" {
			values.Set(k, v)
		}
	}
	var out AuditPage
	if err := c.request(ctx, g, org, "GET", "stores/"+store+"/audit?"+values.Encode(), "", nil, &out); err != nil {
		return out, err
	}
	if out.SchemaVersion != "store-browser-v1" || out.StoreID != store || len(out.Items) > 20 || c.verify(g, org, out.UserID, out.OrganizationID) != nil {
		return AuditPage{}, ErrContext
	}
	return out, nil
}
