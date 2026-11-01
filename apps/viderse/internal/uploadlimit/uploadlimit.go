// Package uploadlimit keeps upload ceilings below the public edge's body limit.
//
// When Viderse is served through a Cloudflare tunnel, the edge rejects any
// request body over 100 MB with its own HTML 413 page before the app runs. The
// app's ceilings (20 GiB for video, 256 MiB for an overlay, 4 GiB for a
// template bundle) are therefore unreachable over the internet, and the user
// sees a Cloudflare error instead of the app's own explanation. Capping the
// hosted ceilings at 90 MiB keeps the app in charge of the refusal and lets it
// say, in Vietnamese, what the limit is.
//
// A local or desktop run (no PUBLIC_URL) has no edge in front: nothing changes
// there, because a 20 GiB import from a local disk is legitimate.
package uploadlimit

import (
	"os"
	"strconv"
	"strings"
)

// PublicMaxBytes is the ceiling used when the app is served over the internet:
// 90 MiB, safely under the edge's 100 MB body limit.
const PublicMaxBytes int64 = 90 << 20

// Hosted reports whether the app is served to other people through a public
// edge. PUBLIC_URL is the same switch the trusted-proxy boundary uses, so the
// two never disagree about which mode the process is in.
func Hosted() bool { return strings.TrimSpace(os.Getenv("PUBLIC_URL")) != "" }

// Cap lowers local to PublicMaxBytes when hosted. A ceiling the operator set
// explicitly through an env override never reaches this function: the caller
// keeps that value verbatim.
func Cap(local int64) int64 {
	if Hosted() && local > PublicMaxBytes {
		return PublicMaxBytes
	}
	return local
}

// Label renders a byte ceiling the way the UI copy states it: "90 MB", "256 MB",
// "4 GB". Whole gigabytes keep the coarser unit; everything else rounds up to a
// whole megabyte, so the stated limit is never larger than the enforced one.
func Label(limit int64) string {
	const (
		mib = int64(1) << 20
		gib = int64(1) << 30
	)
	if limit >= gib && limit%gib == 0 {
		return strconv.FormatInt(limit/gib, 10) + " GB"
	}
	return strconv.FormatInt((limit+mib-1)/mib, 10) + " MB"
}

// TooLargeMessage is the refusal for an upload above a ceiling that is in force
// because the app is public: it names the limit instead of leaving the user to
// guess why a 300 MB clip failed.
func TooLargeMessage(limit int64) string {
	return "Tải lên qua internet tối đa " + Label(limit)
}
