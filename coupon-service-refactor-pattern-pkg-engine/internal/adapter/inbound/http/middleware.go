package http

import (
	"net/http"
	"time"

	mwaudit "github.com/PlatformCore/libpackage/middleware/audit"
	mwcore "github.com/PlatformCore/libpackage/middleware/core"
	mwlogging "github.com/PlatformCore/libpackage/middleware/logging"
	mwmetrics "github.com/PlatformCore/libpackage/middleware/metrics"
	mwrecovery "github.com/PlatformCore/libpackage/middleware/recovery"
	mwrequestid "github.com/PlatformCore/libpackage/middleware/requestid"
	mwsecurity "github.com/PlatformCore/libpackage/middleware/securityheaders"
	mwtimeout "github.com/PlatformCore/libpackage/middleware/timeout"
)

func BuildSharedHTTPMiddleware(serviceName string, timeout time.Duration) func(http.Handler) http.Handler {
	pipeline := mwcore.Compose(
		mwcore.Spec{Name: "request-id", Stage: mwcore.StageIngress, Priority: 5, MW: mwrequestid.Default()},
		mwcore.Spec{Name: "recovery", Stage: mwcore.StageIngress, Priority: 10, MW: mwrecovery.Middleware()},
		mwcore.Spec{Name: "security-headers", Stage: mwcore.StageSecurity, Priority: 10, MW: mwsecurity.Default()},
		mwcore.Spec{Name: "timeout", Stage: mwcore.StageResilience, Priority: 20, MW: mwtimeout.Middleware(timeout)},
		mwcore.Spec{Name: "logging", Stage: mwcore.StageObservability, Priority: 10, MW: mwlogging.Default()},
		mwcore.Spec{Name: "metrics", Stage: mwcore.StageObservability, Priority: 20, MW: mwmetrics.Middleware(&mwmetrics.Options{Namespace: serviceName})},
		mwcore.Spec{Name: "audit", Stage: mwcore.StageObservability, Priority: 30, MW: mwaudit.Default()},
	)
	return mwcore.HTTP(pipeline, "")
}
