package blobkey

import "strings"

// UsageKind owns storage accounting classification alongside the key parser.
// Empty excludes non-library objects; other remains in total without pretending
// to be one of the six known library categories. Voice samples are identified
// by their registry reference, not by guessing an asset path.
func UsageKind(key, contentType string) string {
	object, err := Parse(key)
	if err != nil {
		return "other"
	}
	switch object.Kind {
	case KindAudioSegment, KindAudioChapter:
		return "audio"
	case KindExport:
		return "export"
	case KindAvatar, KindBackupPG:
		return ""
	case KindMusic:
		return "music"
	case KindCover:
		return "cover"
	case KindImport:
		return "import"
	case KindAsset:
		if strings.HasPrefix(contentType, "image/") {
			return "cover"
		}
		if strings.HasPrefix(contentType, "audio/") {
			return "music"
		}
	}
	return "other"
}
