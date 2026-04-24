package authctx

import (
	"github.com/cloudwego/hertz/pkg/app"
)

const UserIDKey = "userID"

func SetUserID(c *app.RequestContext, id int64) {
	c.Set(UserIDKey, id)
}

func UserID(c *app.RequestContext) (int64, bool) {
	v, ok := c.Get(UserIDKey)
	if !ok {
		return 0, false
	}
	id, ok := v.(int64)
	return id, ok
}
