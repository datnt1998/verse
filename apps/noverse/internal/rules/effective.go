package rules

// Effective ghép tại lúc kiểm tra: mặc định hệ thống < gói đang nạp < ý định
// tường minh của người dùng. Snapshot không đóng băng giá trị do gói cung cấp.
func Effective(snapshot *Snapshot, pack Candidate) Structured {
	if pack.Source != "genre_pack" {
		if snapshot != nil {
			return snapshot.Structured
		}
		return SystemDefaults().Structured
	}
	structured := mergeStructured(SystemDefaults().Structured, pack.Structured)
	if snapshot != nil {
		structured = mergeStructured(structured, explicitStructured(*snapshot))
	}
	return structured
}

func explicitStructured(snapshot Snapshot) Structured {
	if snapshot.UserStructured != nil {
		return *snapshot.UserStructured
	}
	// Snapshot cũ đã trộn system_defaults. Những giá trị còn đúng bằng mặc
	// định không chứng minh được ý định ghi đè; giữ các giá trị khác/custom.
	// Ghi đè tường minh bằng đúng mặc định chỉ truy hồi được từ snapshot mới.
	out := mergeStructured(Structured{}, snapshot.Structured)
	for word, limit := range SystemDefaults().Structured.FatigueWords {
		if out.FatigueWords[word] == limit {
			delete(out.FatigueWords, word)
		}
	}
	return out
}
