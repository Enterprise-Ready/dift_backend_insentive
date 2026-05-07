package httpadapter

import (
	_ "github.com/PlatformCore/libpackage/middleware/logging"
	_ "github.com/PlatformCore/libpackage/middleware/metrics"
	_ "github.com/PlatformCore/libpackage/middleware/ratelimit"
	_ "github.com/PlatformCore/libpackage/middleware/recovery"
	_ "github.com/PlatformCore/libpackage/middleware/requestid"
	_ "github.com/PlatformCore/libpackage/middleware/securityheaders"
	_ "github.com/PlatformCore/libpackage/middleware/timeout"
)
