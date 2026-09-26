package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const sensitiveGroupMessageScope = "im:message.group_msg"

var requiredAppScopes = []string{
	"im:message.group_at_msg:readonly",
	"im:message.p2p_msg:readonly",
	"im:message:readonly",
	"im:message:send_as_bot",
	"im:message:update",
	"im:message:recall",
	"im:message.reactions:read",
	"im:message.reactions:write_only",
	"im:chat",
	"im:chat:read",
	"im:chat:update",
	"im:resource",
	"cardkit:card:read",
	"cardkit:card:write",
	"application:application:self_manage",
	"contact:user.base:readonly",
	"docs:doc:readonly",
	"wiki:wiki:readonly",
}

func buildScopeApplyURL(appID string, scopes []string) string {
	if appID == "" {
		return "https://open.feishu.cn/app"
	}
	base := fmt.Sprintf("https://open.feishu.cn/app/%s/auth", appID)
	v := url.Values{}
	if len(scopes) > 0 && len(scopes) < 20 {
		joined := ""
		for i, s := range scopes {
			if i > 0 {
				joined += ","
			}
			joined += s
		}
		v.Set("q", joined)
	}
	v.Set("op_from", "easyagent")
	v.Set("token_type", "tenant")
	return base + "?" + v.Encode()
}

func buildPermissionPageURL(appID string) string {
	if appID == "" {
		return "https://open.feishu.cn/app"
	}
	return fmt.Sprintf("https://open.feishu.cn/app/%s/permission", appID)
}

func buildEventSubURL(appID string) string {
	if appID == "" {
		return "https://open.feishu.cn/app"
	}
	return fmt.Sprintf("https://open.feishu.cn/app/%s/event-sub", appID)
}

func missingScopes(granted []string, required []string) []string {
	set := make(map[string]struct{}, len(granted))
	for _, s := range granted {
		set[s] = struct{}{}
	}
	var missing []string
	for _, s := range required {
		if _, ok := set[s]; !ok {
			missing = append(missing, s)
		}
	}
	return missing
}

func hasScope(granted []string, scope string) bool {
	for _, s := range granted {
		if s == scope {
			return true
		}
	}
	return false
}

// ProbeGrantedScopes returns the application's granted tenant scopes.
// ok=false means the credentials are valid enough for other APIs, but this app
// cannot read its own scope list yet (usually missing application:self_manage).
func (c *Client) ProbeGrantedScopes(ctx context.Context) (scopes []string, ok bool, err error) {
	token, err := c.getTenantToken(ctx)
	if err != nil {
		return nil, false, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://open.feishu.cn/open-apis/application/v6/applications/me?lang=zh_cn", nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	var result struct {
		Code int      `json:"code"`
		Msg  string   `json:"msg"`
		App  scopeApp `json:"app"`
		Data struct {
			App scopeApp `json:"app"`
			scopeApp
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, false, err
	}
	if result.Code != 0 {
		return nil, false, nil
	}

	app := result.Data.App
	if len(app.Scopes) == 0 && len(app.OnlineVersion.Scopes) == 0 {
		app = result.Data.scopeApp
	}
	if len(app.Scopes) == 0 && len(app.OnlineVersion.Scopes) == 0 {
		app = result.App
	}

	for _, s := range app.Scopes {
		if s.Scope != "" {
			scopes = append(scopes, s.Scope)
		}
	}
	if len(scopes) == 0 {
		for _, s := range app.OnlineVersion.Scopes {
			if s.Scope != "" {
				scopes = append(scopes, s.Scope)
			}
		}
	}
	return scopes, true, nil
}

type scopeApp struct {
	Scopes []struct {
		Scope string `json:"scope"`
	} `json:"scopes"`
	OnlineVersion struct {
		Scopes []struct {
			Scope string `json:"scope"`
		} `json:"scopes"`
	} `json:"online_version"`
}
