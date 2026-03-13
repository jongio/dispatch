package styles

import "testing"

// ---------------------------------------------------------------------------
// DeriveTheme benchmarks
// ---------------------------------------------------------------------------

func BenchmarkDeriveTheme(b *testing.B) {
	cs := DispatchDark

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		DeriveTheme(cs)
	}
}

func BenchmarkDeriveThemeLight(b *testing.B) {
	cs := DispatchLight

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		DeriveTheme(cs)
	}
}

func BenchmarkDeriveThemeAllBuiltins(b *testing.B) {
	for name, cs := range BuiltinSchemes {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				DeriveTheme(cs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// contrastText benchmarks
// ---------------------------------------------------------------------------

func BenchmarkContrastText(b *testing.B) {
	// Dark background → expects white text.
	b.Run("dark_bg", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			contrastText("#111111")
		}
	})

	// Light background → expects black text.
	b.Run("light_bg", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			contrastText("#FAFAFA")
		}
	})

	// Mid-range color.
	b.Run("mid_bg", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			contrastText("#5A56E0")
		}
	})
}

// ---------------------------------------------------------------------------
// blendHex benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBlendHex(b *testing.B) {
	b.Run("midpoint", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			blendHex("#111111", "#FAFAFA", 0.5)
		}
	})

	b.Run("quarter", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			blendHex("#DC2626", "#16A34A", 0.25)
		}
	})

	b.Run("identity", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			blendHex("#5A56E0", "#5A56E0", 0.0)
		}
	})
}

// ---------------------------------------------------------------------------
// wcagLuminance benchmarks
// ---------------------------------------------------------------------------

func BenchmarkWcagLuminance(b *testing.B) {
	colors := []struct {
		name string
		hex  string
	}{
		{"black", "#000000"},
		{"white", "#FFFFFF"},
		{"primary_blue", "#5A56E0"},
		{"bright_red", "#F87171"},
	}

	for _, c := range colors {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				wcagLuminance(c.hex)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// contrastRatio benchmarks
// ---------------------------------------------------------------------------

func BenchmarkContrastRatio(b *testing.B) {
	// Pre-compute luminance values to isolate contrastRatio cost.
	l1 := wcagLuminance("#000000")
	l2 := wcagLuminance("#FFFFFF")

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		contrastRatio(l1, l2)
	}
}

// ---------------------------------------------------------------------------
// isDarkHex benchmarks
// ---------------------------------------------------------------------------

func BenchmarkIsDarkHex(b *testing.B) {
	b.Run("dark", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			isDarkHex("#111111")
		}
	})

	b.Run("light", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			isDarkHex("#FAFAFA")
		}
	})
}

// ---------------------------------------------------------------------------
// Palette benchmark
// ---------------------------------------------------------------------------

func BenchmarkPalette(b *testing.B) {
	cs := DispatchDark

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		cs.Palette()
	}
}

// ---------------------------------------------------------------------------
// Validate benchmark
// ---------------------------------------------------------------------------

func BenchmarkValidate(b *testing.B) {
	cs := Campbell

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = cs.Validate()
	}
}
