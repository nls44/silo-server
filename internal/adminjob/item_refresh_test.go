package adminjob

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Silo-Server/silo-server/internal/libraryingest"
	"github.com/Silo-Server/silo-server/internal/models"
	"github.com/Silo-Server/silo-server/internal/scanner"
)

type itemRefreshTestFolderRepo struct {
	folder *models.MediaFolder
	err    error
}

func (r *itemRefreshTestFolderRepo) GetByID(_ context.Context, id int) (*models.MediaFolder, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.folder == nil || r.folder.ID != id {
		return nil, errors.New("folder not found")
	}
	return r.folder, nil
}

func TestItemRefreshResolverBuildRequestAcceptsPathOutsideLibraryRoots(t *testing.T) {
	t.Parallel()

	resolver := &ItemRefreshResolver{
		folderRepo: &itemRefreshTestFolderRepo{
			folder: &models.MediaFolder{
				ID:    3,
				Paths: []string{"/LibraryManager2/movies/popular_trending"},
			},
		},
	}

	req, err := resolver.buildRequest(
		context.Background(),
		3,
		"/srv/media/movies/4k/Example Movie (2026)",
		&ItemRefreshRequest{},
	)
	if err != nil {
		t.Fatalf("buildRequest() error = %v, want nil", err)
	}
	if req == nil {
		t.Fatal("buildRequest() request = nil, want non-nil")
	}
	if got, want := req.ScanPath, "/srv/media/movies/4k/Example Movie (2026)"; got != want {
		t.Fatalf("buildRequest() scan path = %q, want %q", got, want)
	}
}

func TestItemRefreshResolverBuildRequestAcceptsPathWithinLibraryRoots(t *testing.T) {
	t.Parallel()

	resolver := &ItemRefreshResolver{
		folderRepo: &itemRefreshTestFolderRepo{
			folder: &models.MediaFolder{
				ID:    3,
				Paths: []string{"/srv/media/movies"},
			},
		},
	}

	req, err := resolver.buildRequest(
		context.Background(),
		3,
		"/srv/media/movies/4k/Example Movie (2026)",
		&ItemRefreshRequest{RequestedContentID: "119730834381996036"},
	)
	if err != nil {
		t.Fatalf("buildRequest() error = %v, want nil", err)
	}
	if req == nil {
		t.Fatal("buildRequest() request = nil, want non-nil")
	}
	if got, want := req.ScanFolderID, 3; got != want {
		t.Fatalf("buildRequest() folder = %d, want %d", got, want)
	}
	if got, want := req.ScanPath, "/srv/media/movies/4k/Example Movie (2026)"; got != want {
		t.Fatalf("buildRequest() scan path = %q, want %q", got, want)
	}
}

func TestItemRefreshResolverDeriveCompleteRefreshScope_UsesCanonicalRootForSeriesWithoutEpisodes(t *testing.T) {
	t.Parallel()

	resolver := &ItemRefreshResolver{
		folderRepo: &itemRefreshTestFolderRepo{
			folder: &models.MediaFolder{
				ID:   7,
				Type: "series",
			},
		},
	}

	file := &models.MediaFile{
		MediaFolderID:     7,
		FilePath:          "/media/shows/Example Show/Season 01/Example.Show.S01E01.mkv",
		CanonicalRootPath: "/media/shows/Example Show",
		ContentID:         "pending-series",
		BaseType:          "series",
	}

	scanPath, canonicalRootPath, err := resolver.deriveCompleteRefreshScope(context.Background(), file)
	if err != nil {
		t.Fatalf("deriveCompleteRefreshScope() error = %v", err)
	}
	if got, want := scanPath, "/media/shows/Example Show"; got != want {
		t.Fatalf("scanPath = %q, want %q", got, want)
	}
	if got, want := canonicalRootPath, "/media/shows/Example Show"; got != want {
		t.Fatalf("canonicalRootPath = %q, want %q", got, want)
	}
}

type itemRefreshTestIngester struct {
	scanPath string
	result   *libraryingest.Result
}

func (s *itemRefreshTestIngester) IngestSubtree(_ context.Context, _ *models.MediaFolder, subtreePath string) (*libraryingest.Result, error) {
	s.scanPath = subtreePath
	if s.result != nil {
		return s.result, nil
	}
	return &libraryingest.Result{
		ScanResult:   &scanner.ScanResult{},
		MatchedFiles: 2,
	}, nil
}

type itemRefreshTestRefresher struct {
	targetType string
	contentID  string
	folderID   int
}

func (r *itemRefreshTestRefresher) RefreshItem(_ context.Context, contentID string) error {
	r.contentID = contentID
	return nil
}

func (r *itemRefreshTestRefresher) RefreshItemForLibrary(_ context.Context, contentID string, folderID int) error {
	return r.RefreshTargetForLibrary(context.Background(), "item", contentID, folderID)
}

func (r *itemRefreshTestRefresher) RefreshTargetForLibrary(_ context.Context, targetType, contentID string, folderID int) error {
	r.targetType = targetType
	r.contentID = contentID
	r.folderID = folderID
	return nil
}

type itemRefreshTestFileRepo struct {
	filesByPath []*models.MediaFile
	clearedPath string
}

func (r *itemRefreshTestFileRepo) GetByContentID(_ context.Context, _ string) ([]*models.MediaFile, error) {
	return nil, nil
}

func (r *itemRefreshTestFileRepo) GetByEpisodeID(_ context.Context, _ string) ([]*models.MediaFile, error) {
	return nil, nil
}

func (r *itemRefreshTestFileRepo) GetByFolderAndPathPrefix(_ context.Context, _ int, _ string) ([]*models.MediaFile, error) {
	return r.filesByPath, nil
}

func (r *itemRefreshTestFileRepo) ClearContentLinksByPathPrefix(_ context.Context, _ int, pathPrefix string) (int, error) {
	r.clearedPath = pathPrefix
	return 1, nil
}

type itemRefreshTestRootClaimRepo struct {
	deletedRoot string
}

func (r *itemRefreshTestRootClaimRepo) DeleteByFolderAndRoot(_ context.Context, _ int, rootPath string) error {
	r.deletedRoot = rootPath
	return nil
}

type itemRefreshTestGroupClaimRepo struct {
	deletedPath string
}

func (r *itemRefreshTestGroupClaimRepo) DeleteByFolderAndObservedPathPrefix(_ context.Context, _ int, pathPrefix string) error {
	r.deletedPath = pathPrefix
	return nil
}

type itemRefreshTestBackoffResetter struct {
	called   bool
	folderID int
	prefix   string

	seriesCalled   bool
	seriesFolderID int
	seriesPrefix   string
}

func (r *itemRefreshTestBackoffResetter) ResetBackoffByPathPrefix(_ context.Context, folderID int, pathPrefix string) error {
	r.called = true
	r.folderID = folderID
	r.prefix = pathPrefix
	return nil
}

func (r *itemRefreshTestBackoffResetter) ResetBackoffByObservedRootPathPrefix(_ context.Context, folderID int, rootPathPrefix string) error {
	r.seriesCalled = true
	r.seriesFolderID = folderID
	r.seriesPrefix = rootPathPrefix
	return nil
}

type itemRefreshTestSkippedRootRepo struct {
	skipped map[string]models.SkippedMediaRoot
}

func newItemRefreshTestSkippedRootRepo() *itemRefreshTestSkippedRootRepo {
	return &itemRefreshTestSkippedRootRepo{skipped: make(map[string]models.SkippedMediaRoot)}
}

func (r *itemRefreshTestSkippedRootRepo) Upsert(_ context.Context, root models.SkippedMediaRoot) error {
	r.skipped[fmt.Sprintf("%d:%s", root.MediaFolderID, root.RootPath)] = root
	return nil
}

func (r *itemRefreshTestSkippedRootRepo) Delete(_ context.Context, folderID int, rootPath string) error {
	delete(r.skipped, fmt.Sprintf("%d:%s", folderID, rootPath))
	return nil
}

func (r *itemRefreshTestSkippedRootRepo) DeleteMissingInScope(_ context.Context, folderID int, scopePath string, seenRoots []string) error {
	scopePath = filepath.Clean(scopePath)
	prefix := scopePath + string(filepath.Separator)
	seen := make(map[string]struct{}, len(seenRoots))
	for _, root := range seenRoots {
		seen[root] = struct{}{}
	}
	for key, root := range r.skipped {
		if root.MediaFolderID != folderID {
			continue
		}
		if root.RootPath != scopePath && !strings.HasPrefix(root.RootPath, prefix) {
			continue
		}
		if _, ok := seen[root.RootPath]; ok {
			continue
		}
		delete(r.skipped, key)
	}
	return nil
}

type itemRefreshTestSeasonRepo struct {
	season *models.Season
}

func (r *itemRefreshTestSeasonRepo) GetByID(_ context.Context, _ string) (*models.Season, error) {
	return nil, errors.New("not implemented")
}

func (r *itemRefreshTestSeasonRepo) GetBySeriesAndNumber(_ context.Context, seriesID string, seasonNum int) (*models.Season, error) {
	if r.season != nil && r.season.SeriesID == seriesID && r.season.SeasonNumber == seasonNum {
		return r.season, nil
	}
	return nil, errors.New("season not found")
}

type itemRefreshTestEpisodeRepo struct {
	episode *models.Episode
}

func (r *itemRefreshTestEpisodeRepo) GetByID(_ context.Context, _ string) (*models.Episode, error) {
	return nil, errors.New("not implemented")
}

func (r *itemRefreshTestEpisodeRepo) GetBySeriesAndNumber(_ context.Context, seriesID string, seasonNum int, episodeNum int) (*models.Episode, error) {
	if r.episode != nil && r.episode.SeriesID == seriesID && r.episode.SeasonNumber == seasonNum && r.episode.EpisodeNumber == episodeNum {
		return r.episode, nil
	}
	return nil, errors.New("episode not found")
}

func (r *itemRefreshTestEpisodeRepo) ListBySeries(_ context.Context, _ string) ([]*models.Episode, error) {
	return nil, errors.New("not implemented")
}

func (r *itemRefreshTestEpisodeRepo) ListBySeason(_ context.Context, _ string, _ int) ([]*models.Episode, error) {
	return nil, errors.New("not implemented")
}

func (r *itemRefreshTestEpisodeRepo) ListBySeasonID(_ context.Context, _ string) ([]*models.Episode, error) {
	return nil, errors.New("not implemented")
}

func TestItemRefreshExecutorAllowsScanPathOutsideLibraryRoots(t *testing.T) {
	t.Parallel()

	folderRepo := &itemRefreshTestFolderRepo{
		folder: &models.MediaFolder{
			ID:      3,
			Enabled: true,
			Paths:   []string{"/LibraryManager2/movies/popular_trending"},
		},
	}
	ingester := &itemRefreshTestIngester{}
	refresher := &itemRefreshTestRefresher{}
	fileRepo := &itemRefreshTestFileRepo{}
	rootClaimRepo := &itemRefreshTestRootClaimRepo{}
	groupClaimRepo := &itemRefreshTestGroupClaimRepo{}
	skippedRootRepo := newItemRefreshTestSkippedRootRepo()

	executor := NewItemRefreshExecutor(folderRepo, fileRepo, rootClaimRepo, groupClaimRepo, skippedRootRepo, nil, nil, ingester, refresher, nil, nil)

	result, err := executor.Execute(context.Background(), ItemRefreshRequest{
		RequestedContentID: "119730834381996036",
		RefreshContentID:   "119730834381996036",
		ScanFolderID:       3,
		ScanPath:           "/srv/media/movies/4k/Example Movie (2026)",
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Execute() result = nil, want non-nil")
	}
	if got, want := ingester.scanPath, "/srv/media/movies/4k/Example Movie (2026)"; got != want {
		t.Fatalf("IngestSubtree() path = %q, want %q", got, want)
	}
	if got, want := refresher.contentID, "119730834381996036"; got != want {
		t.Fatalf("RefreshItem() content_id = %q, want %q", got, want)
	}
	if got, want := refresher.folderID, 3; got != want {
		t.Fatalf("RefreshItemForLibrary() folder_id = %d, want %d", got, want)
	}
	if got, want := result.MatchedFiles, 2; got != want {
		t.Fatalf("Execute() matched files = %d, want %d", got, want)
	}
}

func TestItemRefreshExecutorCompleteRefreshRebuildsAndMapsEpisodeTarget(t *testing.T) {
	t.Parallel()

	folderRepo := &itemRefreshTestFolderRepo{
		folder: &models.MediaFolder{
			ID:      7,
			Enabled: true,
			Paths:   []string{"/media/mixed"},
		},
	}
	ingester := &itemRefreshTestIngester{}
	refresher := &itemRefreshTestRefresher{}
	fileRepo := &itemRefreshTestFileRepo{
		filesByPath: []*models.MediaFile{
			{ContentID: "new-series-id", FilePath: "/media/mixed/Show/Season 01/Show S01E03.mkv"},
		},
	}
	rootClaimRepo := &itemRefreshTestRootClaimRepo{}
	groupClaimRepo := &itemRefreshTestGroupClaimRepo{}
	episodeRepo := &itemRefreshTestEpisodeRepo{
		episode: &models.Episode{
			ContentID:     "new-episode-id",
			SeriesID:      "new-series-id",
			SeasonNumber:  1,
			EpisodeNumber: 3,
		},
	}

	executor := NewItemRefreshExecutor(
		folderRepo,
		fileRepo,
		rootClaimRepo,
		groupClaimRepo,
		newItemRefreshTestSkippedRootRepo(),
		nil,
		episodeRepo,
		ingester,
		refresher,
		nil,
		nil,
	)

	result, err := executor.Execute(context.Background(), ItemRefreshRequest{
		RequestedContentID:     "old-episode-id",
		RequestedType:          "episode",
		RequestedSeasonNumber:  1,
		RequestedEpisodeNumber: 3,
		RefreshContentID:       "old-series-id",
		ScanFolderID:           7,
		ScanPath:               "/media/mixed/Show/Season 01",
		Mode:                   ItemRefreshModeComplete,
		CanonicalRootPath:      "/media/mixed/Show",
	}, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Execute() result = nil, want non-nil")
	}
	if got, want := fileRepo.clearedPath, "/media/mixed/Show/Season 01"; got != want {
		t.Fatalf("ClearContentLinksByPathPrefix() path = %q, want %q", got, want)
	}
	if got, want := rootClaimRepo.deletedRoot, "/media/mixed/Show"; got != want {
		t.Fatalf("DeleteByFolderAndRoot() root = %q, want %q", got, want)
	}
	if got, want := refresher.targetType, "episode"; got != want {
		t.Fatalf("RefreshTargetForLibrary() target_type = %q, want %q", got, want)
	}
	if got, want := refresher.contentID, "new-episode-id"; got != want {
		t.Fatalf("RefreshItem() content_id = %q, want %q", got, want)
	}
	if got, want := result.RefreshContentID, "new-episode-id"; got != want {
		t.Fatalf("result refresh_content_id = %q, want %q", got, want)
	}
	if got, want := result.DetailContentID, "new-episode-id"; got != want {
		t.Fatalf("result detail_content_id = %q, want %q", got, want)
	}
}

// A complete item refresh must clear accumulated match-queue backoff for its
// scope so the forced re-match actually claims files that had failed before.
func TestItemRefreshExecutor_CompleteRefreshResetsMatchBackoff(t *testing.T) {
	t.Parallel()

	fileRepo := &itemRefreshTestFileRepo{}
	rootClaimRepo := &itemRefreshTestRootClaimRepo{}
	groupClaimRepo := &itemRefreshTestGroupClaimRepo{}
	backoffResetter := &itemRefreshTestBackoffResetter{}

	executor := &ItemRefreshExecutor{
		fileRepo:              fileRepo,
		rootClaimRepo:         rootClaimRepo,
		groupClaimRepo:        groupClaimRepo,
		movieBackoffResetter:  backoffResetter,
		seriesBackoffResetter: backoffResetter,
	}

	if err := executor.prepareCompleteRefresh(
		context.Background(),
		ItemRefreshRequest{
			ScanFolderID:      7,
			ScanPath:          "/media/movies/Cloverfield.1080p.Bluray.x264-1920",
			CanonicalRootPath: "/media/movies/Cloverfield.1080p.Bluray.x264-1920",
		},
	); err != nil {
		t.Fatalf("prepareCompleteRefresh() error = %v", err)
	}

	if !backoffResetter.called {
		t.Fatal("match backoff was not reset during complete refresh")
	}
	if got, want := backoffResetter.folderID, 7; got != want {
		t.Fatalf("backoff reset folder_id = %d, want %d", got, want)
	}
	if got, want := backoffResetter.prefix, "/media/movies/Cloverfield.1080p.Bluray.x264-1920"; got != want {
		t.Fatalf("backoff reset path_prefix = %q, want %q", got, want)
	}
	if !backoffResetter.seriesCalled {
		t.Fatal("series match backoff was not reset during complete refresh")
	}
	if got, want := backoffResetter.seriesFolderID, 7; got != want {
		t.Fatalf("series backoff reset folder_id = %d, want %d", got, want)
	}
	if got, want := backoffResetter.seriesPrefix, "/media/movies/Cloverfield.1080p.Bluray.x264-1920"; got != want {
		t.Fatalf("series backoff reset root_path_prefix = %q, want %q", got, want)
	}
}

// When no backoff resetter is wired, complete refresh must keep working
// (optional dependency) and not panic.
func TestItemRefreshExecutor_CompleteRefreshWithoutBackoffResetter(t *testing.T) {
	t.Parallel()

	executor := &ItemRefreshExecutor{
		fileRepo:       &itemRefreshTestFileRepo{},
		rootClaimRepo:  &itemRefreshTestRootClaimRepo{},
		groupClaimRepo: &itemRefreshTestGroupClaimRepo{},
	}
	if err := executor.prepareCompleteRefresh(
		context.Background(),
		ItemRefreshRequest{ScanFolderID: 7, ScanPath: "/media/movies/X"},
	); err != nil {
		t.Fatalf("prepareCompleteRefresh() error = %v", err)
	}
}
