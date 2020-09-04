package jooki

import (
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }
type JookiSuite struct {}
var _ = Suite(&JookiSuite{})

func (a *JookiSuite) TestX(c *C) {
	c.Check(true, Equals, true)
}
