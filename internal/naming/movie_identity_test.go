package naming

import "testing"

func TestParseInferMovieStem_PrefersBracketedYearOverInTitleNumber(t *testing.T) {
	tests := []struct {
		name        string
		stem        string
		folderTitle string
		folderYear  int
		wantTitle   string
		wantYear    int
	}{
		{
			name:        "automania 2000",
			stem:        "Automania 2000 (1963) [Remux-1080p 8-bit AVC FLAC 2.0]-NiDO",
			folderTitle: "Automania 2000",
			folderYear:  1963,
			wantTitle:   "Automania 2000",
			wantYear:    1963,
		},
		{
			name:        "cherry 2000",
			stem:        "Cherry 2000 (1987) [Remux-1080p 8-bit AVC DTS-HD MA 2.0]-GROUP",
			folderTitle: "Cherry 2000",
			folderYear:  1987,
			wantTitle:   "Cherry 2000",
			wantYear:    1987,
		},
		{
			name:        "class of 1984",
			stem:        "Class of 1984 (1982) [Remux-1080p AVC DTS-HD MA 2.0]-GROUP",
			folderTitle: "Class of 1984",
			folderYear:  1982,
			wantTitle:   "Class of 1984",
			wantYear:    1982,
		},
		{
			name:        "airport 1975",
			stem:        "Airport 1975 (1974) [Remux-1080p AVC DTS-HD MA 2.0]-GROUP",
			folderTitle: "Airport 1975",
			folderYear:  1974,
			wantTitle:   "Airport 1975",
			wantYear:    1974,
		},
		{
			name:        "bracketed year with square brackets",
			stem:        "Equalizer 2000 [1987] [Remux-1080p AVC DTS-HD MA 2.0]-GROUP",
			folderTitle: "Equalizer 2000",
			folderYear:  1987,
			wantTitle:   "Equalizer 2000",
			wantYear:    1987,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stem := parseInferMovieStem(tt.stem, tt.folderTitle, tt.folderYear)
			if stem.Title != tt.wantTitle {
				t.Fatalf("Title = %q, want %q", stem.Title, tt.wantTitle)
			}
			if stem.Year != tt.wantYear {
				t.Fatalf("Year = %d, want %d", stem.Year, tt.wantYear)
			}
		})
	}
}

func TestInferRootAssignments_DoesNotMarkBracketedYearMovieAmbiguous(t *testing.T) {
	roots, _ := InferRootAssignments(
		[]string{
			"/mnt/unionfs/movies/70s/Automania 2000 (1963) {imdb-tt0056841} {tmdb-206998}/Automania 2000 (1963) [Remux-1080p 8-bit AVC FLAC 2.0]-NiDO.mkv",
		},
		"movie",
		1,
		nil,
	)
	if len(roots) != 1 {
		t.Fatalf("len(roots) = %d, want 1", len(roots))
	}
	if got, want := roots[0].State, "resolved"; got != want {
		t.Fatalf("State = %q, want %q", got, want)
	}
	if got, want := roots[0].Title, "Automania 2000"; got != want {
		t.Fatalf("Title = %q, want %q", got, want)
	}
	if got, want := roots[0].Year, 1963; got != want {
		t.Fatalf("Year = %d, want %d", got, want)
	}
}

func TestInferRootAssignments_RecognizesSceneReleaseMovieFolder(t *testing.T) {
	filePath := "/movies/Annabelle.2.Creation.2017.1080p.BluRay.x264-SPARKS/annabelle.2.creation.2017.1080p.bluray.x264-sparks.mkv"
	roots, assignments := InferRootAssignments([]string{filePath}, "movie", 1, nil)
	if len(roots) != 1 {
		t.Fatalf("len(roots) = %d, want 1", len(roots))
	}

	assignment, ok := assignments[filePath]
	if !ok {
		t.Fatal("missing root assignment")
	}
	wantRoot := "/movies/Annabelle.2.Creation.2017.1080p.BluRay.x264-SPARKS"
	if assignment.RootPath != wantRoot {
		t.Fatalf("RootPath = %q, want %q", assignment.RootPath, wantRoot)
	}
	if !assignment.HasMovieEvidence {
		t.Fatal("HasMovieEvidence = false, want true")
	}
	group := InferGroupIdentity(filePath, "movie", assignment)
	if got, want := group.BaseTitle, "Annabelle 2 Creation"; got != want {
		t.Fatalf("BaseTitle = %q, want %q", got, want)
	}
	if got, want := group.BaseYear, 2017; got != want {
		t.Fatalf("BaseYear = %d, want %d", got, want)
	}
}

func TestInferGroupIdentity_DoesNotMarkAmpersandVariantAmbiguous(t *testing.T) {
	group := InferGroupIdentity(
		"/movies/Cowboys and Aliens (2011)/Cowboys & Aliens 2011 Extended Directors Cut BluRay 1080p REMUX AVC DTS-HD MA 5.1-EPSiLON.mkv",
		"movies",
		RootAssignment{
			RootPath:     "/movies/Cowboys and Aliens (2011)",
			InferredType: "movie",
		},
	)

	if got, want := group.State, "resolved"; got != want {
		t.Fatalf("State = %q, want %q", got, want)
	}
	if got, want := group.BaseTitle, "Cowboys and Aliens"; got != want {
		t.Fatalf("BaseTitle = %q, want %q", got, want)
	}
	if got, want := group.BaseYear, 2011; got != want {
		t.Fatalf("BaseYear = %d, want %d", got, want)
	}
}

func TestInferTitlesCoherent_PunctuationAndStyledNumerals(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
	}{
		{
			name:  "ampersand variant",
			left:  "Cowboys and Aliens",
			right: "Cowboys & Aliens",
		},
		{
			name:  "apostrophe variant",
			left:  "Whats Your Number",
			right: "What's Your Number?",
		},
		{
			name:  "styled numeral variant",
			left:  "Alien 3",
			right: "Alien³",
		},
		{
			name:  "comparison safe edition suffix variant",
			left:  "Zack Snyders Justice League Justice Is Gray",
			right: "Zack Snyder's Justice League",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !InferTitlesCoherent(tt.left, tt.right) {
				t.Fatalf("InferTitlesCoherent(%q, %q) = false, want true", tt.left, tt.right)
			}
		})
	}
}

func TestInferGroupIdentity_StripsComparisonSafeEditionSuffixFromBaseTitle(t *testing.T) {
	group := InferGroupIdentity(
		"/movies/Zack Snyders Justice League Justice Is Gray (2021)/Zack.Snyders.Justice.League.Justice.Is.Gray.2021.2160p.HMAX.WEB-DL.DDP5.1.Atmos.HDR.HEVC-TOMMY.mkv",
		"movies",
		RootAssignment{
			RootPath:     "/movies/Zack Snyders Justice League Justice Is Gray (2021)",
			InferredType: "movie",
		},
	)

	if got, want := group.State, "resolved"; got != want {
		t.Fatalf("State = %q, want %q", got, want)
	}
	if got, want := group.BaseTitle, "Zack Snyders Justice League"; got != want {
		t.Fatalf("BaseTitle = %q, want %q", got, want)
	}
}

func TestParseCleanReleaseFolderTitle(t *testing.T) {
	tests := []struct {
		name      string
		folder    string
		wantTitle string
		wantYear  int
		wantOk    bool
	}{
		{name: "scene release no year", folder: "Cloverfield.1080p.Bluray.x264-1920", wantTitle: "Cloverfield", wantYear: 0, wantOk: true},
		{name: "scene release with year", folder: "Cloverfield.2008.1080p.BluRay.x264-1920", wantTitle: "Cloverfield", wantYear: 2008, wantOk: true},
		{name: "title is itself a year", folder: "2012.2009.1080p.BluRay.x264-GROUP", wantTitle: "2012", wantYear: 2009, wantOk: true},
		{name: "title contains yearlike number", folder: "Blade.Runner.2049.2017.2160p.UHD.BluRay", wantTitle: "Blade Runner 2049", wantYear: 2017, wantOk: true},
		{name: "spaces and release group", folder: "The Matrix 1999 1080p REMUX-EPSiLON", wantTitle: "The Matrix", wantYear: 1999, wantOk: true},
		{name: "provider tag stripped before cleaning", folder: "Cloverfield.1080p.BluRay {tmdb-500}", wantTitle: "Cloverfield", wantYear: 0, wantOk: true},
		{name: "trusted bracket year not cleaned", folder: "Cloverfield (2008)", wantTitle: "", wantYear: 0, wantOk: false},
		{name: "plain folder not cleaned", folder: "Cloverfield", wantTitle: "", wantYear: 0, wantOk: false},
		{name: "non release collection folder not cleaned", folder: "4K Movies", wantTitle: "", wantYear: 0, wantOk: false},
		{name: "unrated edition stripped", folder: "Movie.Name.UNRATED.2008.1080p.BluRay.x264-GROUP", wantTitle: "Movie Name", wantYear: 2008, wantOk: true},
		{name: "limited release stripped", folder: "Some.Movie.LIMITED.2007.720p.BluRay.x264-GROUP", wantTitle: "Some Movie", wantYear: 2007, wantOk: true},
		{name: "proper release stripped", folder: "Film.PROPER.2009.1080p.BluRay.x264-GROUP", wantTitle: "Film", wantYear: 2009, wantOk: true},
		{name: "extended edition stripped", folder: "Movie.EXTENDED.2010.1080p.BluRay.x264-GROUP", wantTitle: "Movie", wantYear: 2010, wantOk: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, year, ok := ParseCleanReleaseFolderTitle(tt.folder)
			if title != tt.wantTitle {
				t.Fatalf("title = %q, want %q", title, tt.wantTitle)
			}
			if year != tt.wantYear {
				t.Fatalf("year = %d, want %d", year, tt.wantYear)
			}
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
		})
	}
}

func TestInferGroupIdentity_PrefersCleanedReleaseFolderOverFilename(t *testing.T) {
	group := InferGroupIdentity(
		"/movies/Cloverfield.1080p.Bluray.x264-1920/1920-cloverfield.mkv",
		"movies",
		RootAssignment{
			RootPath:     "/movies/Cloverfield.1080p.Bluray.x264-1920",
			InferredType: "movie",
		},
	)

	if got, want := group.BaseTitle, "Cloverfield"; got != want {
		t.Fatalf("BaseTitle = %q, want %q", got, want)
	}
	if got, want := group.State, "resolved"; got != want {
		t.Fatalf("State = %q, want %q", got, want)
	}
	if got, want := group.Confidence, "medium"; got != want {
		t.Fatalf("Confidence = %q, want %q", got, want)
	}
}

func TestInferGroupIdentity_CleansSceneReleaseFolderTitle(t *testing.T) {
	group := InferGroupIdentity(
		"/movies/Annabelle.2.Creation.2017.1080p.BluRay.x264-SPARKS/annabelle.2.creation.2017.1080p.bluray.x264-sparks.mkv",
		"movies",
		RootAssignment{
			RootPath:     "/movies/Annabelle.2.Creation.2017.1080p.BluRay.x264-SPARKS",
			InferredType: "movie",
		},
	)

	if got, want := group.BaseTitle, "Annabelle 2 Creation"; got != want {
		t.Fatalf("BaseTitle = %q, want %q", got, want)
	}
	if got, want := group.BaseYear, 2017; got != want {
		t.Fatalf("BaseYear = %d, want %d", got, want)
	}
	if got, want := group.State, "resolved"; got != want {
		t.Fatalf("State = %q, want %q", got, want)
	}
}

func TestInferGroupIdentity_BorrowsYearFromFilenameStem(t *testing.T) {
	group := InferGroupIdentity(
		"/movies/Cloverfield.1080p.Bluray.x264-1920/Cloverfield.2008.1080p.BluRay.x264-1920.mkv",
		"movies",
		RootAssignment{
			RootPath:     "/movies/Cloverfield.1080p.Bluray.x264-1920",
			InferredType: "movie",
		},
	)

	if got, want := group.BaseTitle, "Cloverfield"; got != want {
		t.Fatalf("BaseTitle = %q, want %q", got, want)
	}
	if got, want := group.BaseYear, 2008; got != want {
		t.Fatalf("BaseYear = %d, want %d", got, want)
	}
}

func TestParseCleanSeriesReleaseFolderTitle(t *testing.T) {
	tests := []struct {
		name      string
		folder    string
		wantTitle string
		wantYear  int
		wantOk    bool
	}{
		{
			name:      "scene release with season token",
			folder:    "Breaking.Bad.S01.1080p.BluRay.x264-GROUP",
			wantTitle: "Breaking Bad",
			wantYear:  0,
			wantOk:    true,
		},
		{
			name:      "scene release with year and season",
			folder:    "Foundation.2021.S01.1080p.BluRay.x264-GROUP",
			wantTitle: "Foundation",
			wantYear:  2021,
			wantOk:    true,
		},
		{
			name:      "season episode token stripped",
			folder:    "Severance.S01E01.1080p.WEB-DL.DDP5.1.x264-GROUP",
			wantTitle: "Severance",
			wantYear:  0,
			wantOk:    true,
		},
		{
			name:      "season range stripped",
			folder:    "The.Office.S01-S09.1080p.BluRay.x264-GROUP",
			wantTitle: "The Office",
			wantYear:  0,
			wantOk:    true,
		},
		{
			name:      "edition noise stripped before season",
			folder:    "Show.Name.PROPER.S01.1080p.BluRay.x264-GROUP",
			wantTitle: "Show Name",
			wantYear:  0,
			wantOk:    true,
		},
		{
			name:      "plain folder without release tokens not accepted",
			folder:    "Breaking Bad",
			wantTitle: "",
			wantYear:  0,
			wantOk:    false,
		},
		{
			name:      "episode title after SxxExx truncated",
			folder:    "12.Monkeys.S04E01.The.End.1080p.AMZN.WEB-DL.DDP5.1.H.264-NTG",
			wantTitle: "12 Monkeys",
			wantYear:  0,
			wantOk:    true,
		},
		{
			name:      "multi-word episode title truncated",
			folder:    "American.Horror.Story.S09E07.The.Lady.in.White.720p.HDTV.x264-CRiMSON",
			wantTitle: "American Horror Story",
			wantYear:  0,
			wantOk:    true,
		},
		{
			name:      "release tag after SxxExx truncated",
			folder:    "Friends.S01E01.WS.BDRip.XviD-iNGOT",
			wantTitle: "Friends",
			wantYear:  0,
			wantOk:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, year, ok := ParseCleanSeriesReleaseFolderTitle(tt.folder)
			if title != tt.wantTitle {
				t.Fatalf("title = %q, want %q", title, tt.wantTitle)
			}
			if year != tt.wantYear {
				t.Fatalf("year = %d, want %d", year, tt.wantYear)
			}
			if ok != tt.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOk)
			}
		})
	}
}

func TestInferGroupIdentity_SeriesPrefersCleanedReleaseFolder(t *testing.T) {
	group := InferGroupIdentity(
		"/tv/Breaking.Bad.S01.1080p.BluRay.x264-GROUP/Breaking.Bad.S01E01.1080p.BluRay.x264-GROUP.mkv",
		"series",
		RootAssignment{
			RootPath:          "/tv/Breaking.Bad.S01.1080p.BluRay.x264-GROUP",
			InferredType:      "series",
			HasEpisodePattern: true,
		},
	)

	if got, want := group.BaseTitle, "Breaking Bad"; got != want {
		t.Fatalf("BaseTitle = %q, want %q", got, want)
	}
}
