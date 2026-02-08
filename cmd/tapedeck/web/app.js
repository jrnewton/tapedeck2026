// Tapedeck Frontend Application - Mobile Version

const state = {
    stations: [],
    shows: [],
    downloads: [],
    currentDownload: null,
    isPlaying: false,
    offlineIds: new Set(),      // tracks which downloads are saved offline
    downloadingIds: new Set(),  // tracks downloads in progress
    debugMode: localStorage.getItem('debugMode') === 'true',
    // Downloads page state
    currentPage: 'main',        // 'main' or 'downloads'
    allShows: [],               // all shows from adapter (for downloads page)
    schedules: []               // scheduled downloads
};

// Debug helpers - only log/alert when debug mode is enabled
function debugLog(...args) {
    if (state.debugMode) {
        console.log('[DEBUG]', ...args);
    }
}

function debugWarn(...args) {
    if (state.debugMode) {
        console.warn('[DEBUG]', ...args);
    }
}

function debugError(...args) {
    if (state.debugMode) {
        console.error('[DEBUG]', ...args);
    }
}

function debugAlert(message) {
    debugLog(message);
    if (state.debugMode) {
        alert(message);
    }
}

// Show error modal with message (visible to all users, not just debug mode)
function showErrorModal(message) {
    const errorModal = document.getElementById('error-modal');
    const errorModalMessage = document.getElementById('error-modal-message');
    if (errorModal && errorModalMessage) {
        errorModalMessage.textContent = message;
        errorModal.classList.remove('hidden');
    }
}

// Handle 401 Unauthorized - redirect to OAuth login with pending action context
function handle401(pendingAction) {
    const params = new URLSearchParams();
    params.set('page', 'downloads');
    if (pendingAction) {
        params.set('action', pendingAction.type);
        if (pendingAction.station) params.set('dl_station', pendingAction.station);
        if (pendingAction.show) params.set('dl_show', pendingAction.show);
        if (pendingAction.scheduleId) params.set('schedule_id', String(pendingAction.scheduleId));
    }
    const rd = '/?' + params.toString();
    window.location.href = '/oauth2/sign_in?rd=' + encodeURIComponent(rd);
}

// Replay a pending action after returning from OAuth login
async function replayPendingAction(params) {
    const action = params.get('action');
    if (!action) return;

    const station = params.get('dl_station');
    const show = params.get('dl_show');
    const scheduleId = params.get('schedule_id');

    // Clean action params from URL
    params.delete('action');
    params.delete('dl_station');
    params.delete('dl_show');
    params.delete('schedule_id');
    updateURL(params);

    // Restore dropdown state for schedule/download actions
    if ((action === 'schedule' || action === 'download') && station && show) {
        dlStationSelect.value = station;
        await loadAllShows(station);
        renderAllShowsDropdown(dlShowSelect);
        dlShowSelect.value = show;
    }

    // Replay the action
    if (action === 'schedule') {
        await createSchedule();
    } else if (action === 'download') {
        await queueDownload();
    } else if (action === 'delete-schedule' && scheduleId) {
        await deleteSchedule(Number(scheduleId));
    }
}

// Update page title based on current state
function updatePageTitle() {
    const base = 'Tapedeck';

    if (state.currentDownload) {
        const d = state.currentDownload;
        const date = new Date(d.ArchiveDate);
        const dateStr = date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', timeZone: 'UTC' });
        document.title = `${base} - ${d.Show} · ${dateStr}`;
        return;
    }

    const showOption = showSelect.options[showSelect.selectedIndex];
    if (showSelect.value && showOption) {
        document.title = `${base} - ${showOption.textContent}`;
        return;
    }

    if (stationSelect.value) {
        document.title = `${base} - ${stationSelect.value}`;
        return;
    }

    document.title = base;
}

// URL State Management
function getURLParams() {
    return new URLSearchParams(window.location.search);
}

function updateURL(params) {
    const url = new URL(window.location);
    url.search = params.toString();
    history.pushState({}, '', url);
}

async function applyURLState() {
    const params = getURLParams();
    const page = params.get('page');
    const station = params.get('station');
    const showId = params.get('show');
    const playId = params.get('play');

    // Handle page switching from URL
    if (page === 'downloads') {
        await showPage('downloads');
        await replayPendingAction(params);
        return;
    }

    // Ensure we're on main page
    if (state.currentPage !== 'main') {
        mainView.classList.remove('hidden');
        miniPlayer.classList.remove('hidden');
        downloadsView.classList.add('hidden');
        state.currentPage = 'main';
    }

    if (station) {
        stationSelect.value = station;
        await loadShows(station);

        if (showId) {
            showSelect.value = showId;
            if (showSelect.value === showId) {
                await loadDownloads(showId);

                if (playId) {
                    const download = state.downloads.find(d => d.ID === Number(playId));
                    if (download) await loadDownloadWithoutPlay(download); // Load but don't autoplay
                }
            } else {
                // Show no longer exists in dropdown - clean URL
                const params = getURLParams();
                params.delete('show');
                params.delete('play');
                updateURL(params);
            }
        }
    }
    updatePageTitle();
}

// DOM Elements
const stationSelect = document.getElementById('station-select');
const showSelect = document.getElementById('show-select');
const tapeList = document.getElementById('tape-list');
const audioPlayer = document.getElementById('audio-player');
const nowPlaying = document.getElementById('now-playing');
const btnPlay = document.getElementById('btn-play');
const btnBack = document.getElementById('btn-back');
const btnFwd = document.getElementById('btn-fwd');
const btnHelp = document.getElementById('btn-help');
const progressBar = document.getElementById('progress-bar');
const timeCurrent = document.getElementById('time-current');
const timeTotal = document.getElementById('time-total');
const leftReel = document.querySelector('.left-reel');
const rightReel = document.querySelector('.right-reel');
const aboutModal = document.getElementById('about-modal');
const modalClose = document.getElementById('modal-close');
const errorModal = document.getElementById('error-modal');
const errorModalClose = document.getElementById('error-modal-close');
// errorModalMessage is accessed inside showErrorModal function

// Downloads page DOM elements
const mainView = document.getElementById('main-view');
const downloadsView = document.getElementById('downloads-view');
const miniPlayer = document.querySelector('.mini-player');
const btnRecord = document.getElementById('btn-record');
const backBtn = document.getElementById('back-btn');
const dlStationSelect = document.getElementById('dl-station-select');
const dlShowSelect = document.getElementById('dl-show-select');
const downloadBtn = document.getElementById('download-btn');
const scheduleBtn = document.getElementById('schedule-btn');
const schedulesList = document.getElementById('schedules-list');
const downloadOverlay = document.getElementById('download-overlay');
const downloadOverlayIcon = document.getElementById('download-overlay-icon');

// Initialize
async function init() {
    // Load offline IDs from IndexedDB
    await loadOfflineIds();

    await loadStations();
    setupEventListeners();
    await applyURLState();

    // Display app version from SW cache name (pick highest version)
    try {
        const names = await caches.keys();
        const versions = names
            .filter(n => n.startsWith('tapedeck-v'))
            .map(n => Number(n.replace('tapedeck-v', '')))
            .filter(n => !isNaN(n));
        if (versions.length > 0) {
            document.getElementById('app-version').textContent = 'v' + Math.max(...versions);
        }
    } catch (_e) { /* caches API unavailable */ }
}

// Load offline download IDs from IndexedDB
async function loadOfflineIds() {
    try {
        const ids = await window.offlineStorage.listOfflineIds();
        state.offlineIds = new Set(ids);
    } catch (error) {
        debugWarn('Failed to load offline IDs:', error);
        state.offlineIds = new Set();
    }
}

// API Functions

// Cache-first: return cached data immediately, refresh in background
async function fetchJSON(url) {
    const cacheKey = `api-cache:${url}`;
    const cached = localStorage.getItem(cacheKey);

    // Cache-first: return cached data immediately if available
    if (cached) {
        // Refresh cache in background (don't await)
        refreshCache(url, cacheKey);
        return JSON.parse(cached);
    }

    // No cache: must fetch from network
    return fetchAndCache(url, cacheKey);
}

// Background refresh - update cache and re-render if data changed
async function refreshCache(url, cacheKey) {
    try {
        const response = await fetch(url);
        if (response.ok) {
            const data = await response.json();
            const newJson = JSON.stringify(data);
            const oldJson = localStorage.getItem(cacheKey);

            // Only update and re-render if data actually changed
            if (newJson !== oldJson) {
                localStorage.setItem(cacheKey, newJson);

                // Re-render the relevant UI component
                const handler = getCacheRefreshHandler(url);
                if (handler) {
                    handler(data);
                }
            }
        }
    } catch (_e) {
        // Silently fail - we already returned cached data
    }
}

// Get handler for re-rendering when cache is refreshed with new data
function getCacheRefreshHandler(url) {
    if (url === '/api/stations') {
        return (data) => {
            state.stations = data;
            renderStations();
        };
    }
    if (url.match(/^\/api\/stations\/[^/]+\/shows$/)) {
        return (data) => {
            const selectedShow = showSelect.value;
            state.shows = data;
            renderShows();
            // Preserve selection if show still exists, otherwise clear stale state
            if (selectedShow && data.some(s => String(s.ID) === selectedShow)) {
                showSelect.value = selectedShow;
            } else if (selectedShow) {
                // Selected show no longer exists - clear all stale state
                showSelect.value = '';
                state.downloads = [];
                state.currentDownload = null;
                renderDownloads();
                updateNowPlaying();
                const params = getURLParams();
                params.delete('show');
                params.delete('play');
                updateURL(params);
                updatePageTitle();
            }
        };
    }
    if (url.match(/^\/api\/shows\/\d+\/downloads/)) {
        return (data) => {
            state.downloads = data;
            renderDownloads();
            // Clear play state if current download no longer exists
            if (state.currentDownload && !data.some(d => d.ID === state.currentDownload.ID)) {
                state.currentDownload = null;
                updateNowPlaying();
                const params = getURLParams();
                params.delete('play');
                updateURL(params);
            }
        };
    }
    return null;
}

// Fetch from network and cache the result
async function fetchAndCache(url, cacheKey) {
    const response = await fetch(url);
    if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    const data = await response.json();
    try {
        localStorage.setItem(cacheKey, JSON.stringify(data));
    } catch (_e) {
        // localStorage might be full
    }
    return data;
}

async function loadStations() {
    try {
        state.stations = await fetchJSON('/api/stations');
        renderStations();
    } catch (error) {
        debugError('Failed to load stations:', error);
        state.stations = [];
        renderStations();
    }
}

async function loadShows(callSign) {
    try {
        state.shows = await fetchJSON(`/api/stations/${callSign}/shows`);
        renderShows();
    } catch (error) {
        debugError('Failed to load shows:', error);
        state.shows = [];
        renderShows();
    }
}

async function loadDownloads(showId) {
    try {
        state.downloads = await fetchJSON(`/api/shows/${showId}/downloads?status=completed`);
        renderDownloads();
    } catch (error) {
        debugError('Failed to load downloads:', error);
        state.downloads = [];
        renderDownloads();
    }
}

// Render Functions
function renderStations() {
    stationSelect.innerHTML = '<option value="">Select station...</option>';
    state.stations.forEach(station => {
        const option = document.createElement('option');
        option.value = station.CallSign;
        option.textContent = station.CallSign + (station.Name ? ` - ${station.Name}` : '');
        stationSelect.appendChild(option);
    });
}

function renderShows() {
    showSelect.innerHTML = '<option value="">Select show...</option>';
    showSelect.disabled = state.shows.length === 0;

    state.shows.forEach(show => {
        const option = document.createElement('option');
        option.value = show.ID;
        option.textContent = show.Name;
        showSelect.appendChild(option);
    });
}

function renderDownloads() {
    if (!state.downloads || state.downloads.length === 0) {
        let msg;
        if (showSelect.value) {
            msg = 'No completed downloads for this show';
        } else if (stationSelect.value) {
            msg = 'Select a show to view downloads';
        } else {
            msg = 'Select a station';
        }
        tapeList.innerHTML = `<p class="empty-message">${msg}</p>`;
        return;
    }

    tapeList.innerHTML = '';
    state.downloads.forEach(download => {
        const spine = document.createElement('div');
        spine.className = 'tape-spine';
        if (state.currentDownload && state.currentDownload.ID === download.ID) {
            spine.classList.add('active');
        }
        spine.dataset.id = download.ID;

        const date = new Date(download.ArchiveDate);
        const dateStr = date.toLocaleDateString('en-US', {
            weekday: 'short',
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            timeZone: 'UTC'
        }).replace(/,/g, '');

        // Determine offline button state
        const isOffline = state.offlineIds.has(download.ID);
        const isDownloading = state.downloadingIds.has(download.ID);
        let btnClass = 'offline-btn';
        const svgIcon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3v12M12 15l-4-4M12 15l4-4"/><path d="M4 17v2a2 2 0 002 2h12a2 2 0 002-2v-2"/></svg>';
        let btnContent = svgIcon;
        if (isDownloading) {
            btnClass += ' downloading';
            btnContent = ''; // Spinner via CSS
        } else if (isOffline) {
            btnClass += ' saved';
            btnContent = svgIcon; // Same icon, color changes via CSS
        }

        spine.innerHTML = `
            <div class="tape-info">
                <div class="tape-date">${dateStr}</div>
                <div class="tape-show">${download.Station} - ${download.Show}</div>
            </div>
            <button class="${btnClass}" title="${isOffline ? 'Remove from device' : 'Save to device'}">${btnContent}</button>
        `;

        // Play on tape info click (not button)
        const tapeInfo = spine.querySelector('.tape-info');
        tapeInfo.addEventListener('click', () => playDownload(download));

        // Offline button click
        const offlineBtn = spine.querySelector('.offline-btn');
        offlineBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            toggleOffline(download);
        });

        tapeList.appendChild(spine);
    });
}

// Offline Storage Functions

// Toggle offline status for a download
async function toggleOffline(download) {
    if (state.downloadingIds.has(download.ID)) {
        return; // Already in progress
    }

    if (state.offlineIds.has(download.ID)) {
        // Remove from offline storage
        try {
            await window.offlineStorage.deleteAudio(download.ID);
            state.offlineIds.delete(download.ID);
            renderDownloads();
        } catch (error) {
            debugError('Failed to remove offline audio:', error);
        }
    } else {
        // Save for offline
        await saveForOffline(download);
    }
}

// Fetch and save audio to IndexedDB
async function saveForOffline(download) {
    state.downloadingIds.add(download.ID);
    renderDownloads();

    try {
        const response = await fetch(`/api/audio/${download.ID}`);
        if (!response.ok) {
            throw new Error(`Fetch failed: HTTP ${response.status}`);
        }

        const blob = await response.blob();
        const sizeMB = (blob.size / 1024 / 1024).toFixed(1);
        debugLog(`Downloaded ${sizeMB}MB blob for ID ${download.ID}`);

        const metadata = {
            station: download.Station,
            show: download.Show,
            archiveDate: download.ArchiveDate
        };

        await window.offlineStorage.saveAudio(download.ID, metadata, blob);
        state.offlineIds.add(download.ID);
        debugLog(`Saved to IndexedDB: ID ${download.ID}`);
    } catch (error) {
        debugError('Failed to save audio offline:', error);
        debugAlert('Download failed: ' + error.message);
    } finally {
        state.downloadingIds.delete(download.ID);
        renderDownloads();
    }
}

// Get audio source - returns Blob URL if offline, otherwise API URL
async function getAudioSource(download) {
    if (state.offlineIds.has(download.ID)) {
        try {
            const record = await window.offlineStorage.getAudio(download.ID);
            if (record && record.blob) {
                const blobUrl = URL.createObjectURL(record.blob);
                debugLog('Playing from offline storage:', blobUrl);
                return blobUrl;
            }
            debugWarn('Offline record found but no blob for ID:', download.ID);
        } catch (error) {
            debugWarn('Failed to load offline audio, falling back to network:', error);
        }
    }
    debugLog('Playing from network:', `/api/audio/${download.ID}`);
    return `/api/audio/${download.ID}`;
}

// Playback Functions

// Load a download without playing - used when restoring from URL
async function loadDownloadWithoutPlay(download) {
    state.currentDownload = download;
    audioPlayer.src = await getAudioSource(download);
    audioPlayer.load();
    state.isPlaying = false;
    updateNowPlaying();
    updatePlayButton();
    updatePageTitle();
    renderDownloads(); // Update active state
}

async function playDownload(download, shouldUpdateURL = true) {
    state.currentDownload = download;
    const src = await getAudioSource(download);
    const isOffline = src.startsWith('blob:');
    audioPlayer.src = src;
    audioPlayer.load();
    try {
        await audioPlayer.play();
    } catch (error) {
        debugError('Playback failed:', error.name, error.message);
        const mode = isOffline ? 'offline blob' : 'network';
        debugAlert(`Playback failed (${mode}): ${error.message}`);
    }
    state.isPlaying = true;
    updateNowPlaying();
    updatePlayButton();
    updatePageTitle();
    startReels();
    renderDownloads(); // Update active state

    if (shouldUpdateURL) {
        const params = getURLParams();
        params.set('play', download.ID);
        updateURL(params);
    }
}

function updateNowPlaying() {
    if (!state.currentDownload) {
        nowPlaying.textContent = 'No tape loaded';
        return;
    }

    const download = state.currentDownload;
    const date = new Date(download.ArchiveDate);
    const dateStr = date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' }).replace(/,/g, '');
    nowPlaying.textContent = `${download.Show} · ${dateStr}`;
}

function updatePlayButton() {
    const playIcon = btnPlay.querySelector('.play-icon');
    const pauseIcon = btnPlay.querySelector('.pause-icon');
    if (state.isPlaying) {
        playIcon.classList.add('hidden');
        pauseIcon.classList.remove('hidden');
    } else {
        playIcon.classList.remove('hidden');
        pauseIcon.classList.add('hidden');
    }
}

async function togglePlay() {
    if (!state.currentDownload) {
        // If no download selected, play first one
        if (state.downloads && state.downloads.length > 0) {
            playDownload(state.downloads[0]);
        }
        return;
    }

    if (state.isPlaying) {
        audioPlayer.pause();
        state.isPlaying = false;
        stopReels();
    } else {
        try {
            await audioPlayer.play();
            state.isPlaying = true;
            startReels();
        } catch (error) {
            debugError('Playback failed:', error.name, error.message);
            debugAlert('Playback failed: ' + error.message);
        }
    }
    updatePlayButton();
}

function playNext() {
    if (!state.currentDownload || state.downloads.length === 0) return;

    const currentIndex = state.downloads.findIndex(d => d.ID === state.currentDownload.ID);
    const nextIndex = (currentIndex + 1) % state.downloads.length;
    playDownload(state.downloads.at(nextIndex));
}

function startReels() {
    leftReel.classList.add('spinning');
    rightReel.classList.add('spinning');
}

function stopReels() {
    leftReel.classList.remove('spinning');
    rightReel.classList.remove('spinning');
}

function formatTime(seconds) {
    if (isNaN(seconds)) return '0:00';
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

// =====================================================
// Downloads Page Functions
// =====================================================

// Show/hide pages
async function showPage(page) {
    state.currentPage = page;

    if (page === 'downloads') {
        mainView.classList.add('hidden');
        miniPlayer.classList.add('hidden');
        downloadsView.classList.remove('hidden');
        document.title = 'Tapedeck downloads';
        // Reset show dropdown to default
        dlShowSelect.value = '';
        // Update URL
        const params = getURLParams();
        params.set('page', 'downloads');
        updateURL(params);
        // Load data
        await loadDownloadsPageData();
    } else {
        downloadsView.classList.add('hidden');
        mainView.classList.remove('hidden');
        miniPlayer.classList.remove('hidden');
        document.activeElement.blur(); // Clear focus to prevent stuck hover on touch
        updatePageTitle(); // Restore main page title
        // Remove page param from URL
        const params = getURLParams();
        params.delete('page');
        updateURL(params);
        // Refresh shows dropdown with updated cache, preserving selection
        if (stationSelect.value) {
            const showId = params.get('show');
            await loadShows(stationSelect.value);
            if (showId) {
                showSelect.value = showId;
            }
        }
    }
}

// Load all data needed for downloads page
async function loadDownloadsPageData() {
    // Populate station select with registered stations
    await populateDownloadStations();
    // Load schedules
    await loadSchedules();
}

// Populate station dropdown on downloads page
async function populateDownloadStations() {
    // Use the same stations from main page
    if (state.stations.length === 0) {
        await loadStations();
    }

    // Populate station select (using DOM methods to avoid innerHTML)
    while (dlStationSelect.firstChild) {
        dlStationSelect.removeChild(dlStationSelect.firstChild);
    }
    const defaultOption = document.createElement('option');
    defaultOption.value = '';
    defaultOption.textContent = 'Select station...';
    dlStationSelect.appendChild(defaultOption);

    state.stations.forEach(station => {
        const option = document.createElement('option');
        option.value = station.CallSign;
        option.textContent = station.CallSign + (station.Name ? ` - ${station.Name}` : '');
        dlStationSelect.appendChild(option);
    });
}

// Load all shows for a station (from adapter, not just downloaded)
async function loadAllShows(callSign) {
    try {
        const response = await fetch(`/api/stations/${callSign}/allshows`);
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }
        state.allShows = await response.json();
        return state.allShows;
    } catch (error) {
        debugError('Failed to load all shows:', error);
        state.allShows = [];
        return [];
    }
}

// Render shows dropdown with all shows
function renderAllShowsDropdown(selectElement) {
    selectElement.innerHTML = '<option value="">Select show...</option>';
    selectElement.disabled = state.allShows.length === 0;

    state.allShows.forEach(showName => {
        const option = document.createElement('option');
        option.value = showName;
        option.textContent = showName;
        selectElement.appendChild(option);
    });
}

// Queue a download
async function queueDownload() {
    const station = dlStationSelect.value;
    const show = dlShowSelect.value;

    if (!station || !show) {
        debugAlert('Please select a station and show');
        return;
    }

    // Check if this is a new show (not in main page shows list)
    const isNewShow = !state.shows.some(s => s.Name === show);
    debugLog('queueDownload: show=', show, 'isNewShow=', isNewShow, 'state.shows=', state.shows.map(s => s.Name));

    const date = 'latest';

    // Disable both buttons while download is in progress
    downloadBtn.disabled = true;
    scheduleBtn.disabled = true;

    // Show download overlay
    downloadOverlay.classList.remove('hidden');
    downloadOverlayIcon.className = 'download-icon downloading';
    downloadOverlayIcon.textContent = '';

    try {
        const response = await fetch('/api/downloads', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ station, show, date })
        });

        if (response.status === 401) {
            handle401({ type: 'download', station, show });
            return;
        } else if (response.status === 409) {
            showErrorModal('This episode is already downloaded or queued');
            showDownloadResult(false, null);
        } else if (!response.ok) {
            const error = await response.text();
            showErrorModal('Failed to queue download: ' + error);
            showDownloadResult(false, null);
        } else {
            const download = await response.json();
            debugLog('Download queued successfully, ID:', download.ID);
            // Start polling for completion
            // Pass station if this is a new show (to refresh cache on success)
            const stationToRefresh = isNewShow ? station : null;
            debugLog('pollDownloadStatus: stationToRefresh=', stationToRefresh);
            pollDownloadStatus(download.ID, stationToRefresh);
        }
    } catch (error) {
        debugError('Failed to queue download:', error);
        showErrorModal('Failed to queue download: ' + error.message);
        showDownloadResult(false, null);
    }
}

// Poll for download completion status
// stationToRefresh: if set, refresh shows cache for this station on success
async function pollDownloadStatus(downloadId, stationToRefresh) {
    const poll = async () => {
        try {
            const response = await fetch(`/api/downloads/${downloadId}`);
            if (!response.ok) {
                debugError('Failed to poll download status:', response.status);
                showDownloadResult(false, null);
                return;
            }

            const download = await response.json();
            debugLog('Download status:', download.Status);

            if (download.Status === 'completed') {
                showDownloadResult(true, stationToRefresh);
            } else if (download.Status === 'failed') {
                showDownloadResult(false, null);
            } else {
                // Still pending or downloading, poll again
                setTimeout(poll, 2000);
            }
        } catch (error) {
            debugError('Error polling download status:', error);
            showDownloadResult(false, null);
        }
    };
    poll();
}

// Show download result and re-enable buttons
// stationToRefresh: if set, invalidate and pre-fetch shows cache for this station
function showDownloadResult(success, stationToRefresh) {
    debugLog('showDownloadResult: success=', success, 'stationToRefresh=', stationToRefresh);

    if (success) {
        downloadOverlayIcon.className = 'download-icon success';
        downloadOverlayIcon.textContent = '✔';

        // If this was a new show, invalidate cache and pre-fetch
        if (stationToRefresh) {
            const cacheKey = `api-cache:/api/stations/${stationToRefresh}/shows`;
            debugLog('Invalidating cache and pre-fetching:', cacheKey);
            localStorage.removeItem(cacheKey);
            // Pre-fetch in background so data is ready when user returns to main page
            fetchAndCache(`/api/stations/${stationToRefresh}/shows`, cacheKey);
        }
    } else {
        downloadOverlayIcon.className = 'download-icon error';
        downloadOverlayIcon.textContent = '✖';
    }

    // Auto-dismiss overlay after delay
    setTimeout(() => {
        downloadOverlay.classList.add('hidden');
        downloadBtn.disabled = false;
        scheduleBtn.disabled = false;
    }, 1000);
}

// Load schedules
async function loadSchedules() {
    try {
        const response = await fetch('/api/schedules');
        if (!response.ok) {
            // Check for offline (503 from service worker)
            if (response.status === 503) {
                showErrorModal('Downloads page requires server connection');
            }
            throw new Error(`HTTP ${response.status}`);
        }
        const data = await response.json();
        state.schedules = data.schedules || [];
        renderSchedules();
    } catch (error) {
        debugError('Failed to load schedules:', error);
        // Show offline modal for network errors (fetch threw before returning response)
        if (error.message.includes('Failed to fetch') || error.message.includes('NetworkError')) {
            showErrorModal('Downloads page requires server connection');
        }
        state.schedules = [];
        renderSchedules();
    }
}

// Render schedules list
function renderSchedules() {
    // Clear existing content safely
    while (schedulesList.firstChild) {
        schedulesList.removeChild(schedulesList.firstChild);
    }

    if (state.schedules.length === 0) {
        const emptyMsg = document.createElement('p');
        emptyMsg.className = 'empty-message';
        emptyMsg.textContent = 'No scheduled downloads';
        schedulesList.appendChild(emptyMsg);
        return;
    }

    state.schedules.forEach(sched => {
        const card = document.createElement('div');
        card.className = 'schedule-card';

        const schedInfo = document.createElement('div');
        schedInfo.className = 'schedule-info';

        const schedShow = document.createElement('div');
        schedShow.className = 'schedule-show';
        schedShow.textContent = `${sched.Station} - ${sched.Show}`;

        const schedCron = document.createElement('div');
        schedCron.className = 'schedule-cron';
        // Use backend-provided CronDescription
        schedCron.textContent = sched.CronDescription;

        const schedTimes = document.createElement('div');
        schedTimes.className = 'schedule-times';
        // Use backend-provided display strings, with fallbacks
        const lastRun = sched.LastRunDisplay && sched.LastRunDisplay !== '-'
            ? sched.LastRunDisplay
            : (sched.LastRunAt
                ? new Date(sched.LastRunAt).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
                : 'Never');
        const nextRun = sched.NextRunDisplay && sched.NextRunDisplay !== '-'
            ? sched.NextRunDisplay
            : (sched.NextRunAt
                ? new Date(sched.NextRunAt).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
                : 'N/A');
        schedTimes.textContent = `Last: ${lastRun} · Next: ${nextRun}`;

        schedInfo.appendChild(schedShow);
        schedInfo.appendChild(schedCron);
        schedInfo.appendChild(schedTimes);

        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'schedule-delete';
        deleteBtn.title = 'Delete schedule';
        deleteBtn.textContent = '\u00D7'; // ×
        deleteBtn.addEventListener('click', () => deleteSchedule(sched.ID));

        card.appendChild(schedInfo);
        card.appendChild(deleteBtn);
        schedulesList.appendChild(card);
    });
}

// Create a schedule
async function createSchedule() {
    const station = dlStationSelect.value;
    const show = dlShowSelect.value;

    if (!station || !show) {
        debugAlert('Please select a station and show');
        return;
    }

    scheduleBtn.disabled = true;
    scheduleBtn.classList.add('queued');
    scheduleBtn.textContent = 'SAVING...';

    let success = false;
    try {
        const response = await fetch('/api/schedules', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ station, show })
        });

        if (response.status === 401) {
            handle401({ type: 'schedule', station, show });
            return;
        } else if (response.status === 409) {
            showErrorModal('A schedule already exists for this show');
        } else if (!response.ok) {
            const error = await response.text();
            showErrorModal('Failed to create schedule: ' + error);
        } else {
            debugLog('Schedule created successfully');
            success = true;
        }

        // Refresh schedules
        await loadSchedules();
    } catch (error) {
        debugError('Failed to create schedule:', error);
        showErrorModal('Failed to create schedule: ' + error.message);
    }

    // Show success/error briefly, then reset
    scheduleBtn.classList.remove('queued');
    if (success) {
        scheduleBtn.classList.add('success');
        scheduleBtn.textContent = '\u2713'; // ✓
    } else {
        scheduleBtn.classList.add('error');
        scheduleBtn.textContent = '\u2717'; // ✗
    }

    setTimeout(() => {
        scheduleBtn.disabled = false;
        scheduleBtn.classList.remove('success', 'error');
        scheduleBtn.textContent = 'SCHEDULE';
    }, 1500);
}

// Delete a schedule
async function deleteSchedule(id) {
    try {
        const response = await fetch(`/api/schedules/${id}`, {
            method: 'DELETE'
        });

        if (response.status === 401) {
            handle401({ type: 'delete-schedule', scheduleId: id });
            return;
        } else if (!response.ok) {
            const error = await response.text();
            showErrorModal('Failed to delete schedule: ' + error);
        } else {
            debugLog('Schedule deleted');
        }

        // Refresh schedules
        await loadSchedules();
    } catch (error) {
        debugError('Failed to delete schedule:', error);
        showErrorModal('Failed to delete schedule: ' + error.message);
    }
}


// Event Listeners
function setupEventListeners() {
    // Debug toggle
    const debugToggle = document.getElementById('debug-toggle');
    if (state.debugMode) {
        debugToggle.classList.add('active');
    }
    debugToggle.addEventListener('click', () => {
        state.debugMode = !state.debugMode;
        localStorage.setItem('debugMode', state.debugMode);
        debugToggle.classList.toggle('active', state.debugMode);
    });

    // About modal
    modalClose.addEventListener('click', () => {
        aboutModal.classList.add('hidden');
    });

    aboutModal.addEventListener('click', (e) => {
        if (e.target === aboutModal) {
            aboutModal.classList.add('hidden');
        }
    });

    // Error modal
    errorModalClose.addEventListener('click', () => {
        errorModal.classList.add('hidden');
    });

    errorModal.addEventListener('click', (e) => {
        if (e.target === errorModal) {
            errorModal.classList.add('hidden');
        }
    });

    stationSelect.addEventListener('change', async (e) => {
        const callSign = e.target.value;
        if (callSign) {
            await loadShows(callSign);
            // Update URL with station, remove show and play
            const params = new URLSearchParams();
            params.set('station', callSign);
            updateURL(params);
        } else {
            state.shows = [];
            renderShows();
            // Clear URL params
            updateURL(new URLSearchParams());
        }
        state.downloads = [];
        state.currentDownload = null;
        renderDownloads();
        updatePageTitle();
    });

    showSelect.addEventListener('change', async (e) => {
        const showId = e.target.value;
        if (showId) {
            await loadDownloads(showId);
            // Update URL with station and show, remove play
            const params = new URLSearchParams();
            const station = stationSelect.value;
            if (station) params.set('station', station);
            params.set('show', showId);
            updateURL(params);
        } else {
            state.downloads = [];
            renderDownloads();
            // Keep only station in URL
            const params = new URLSearchParams();
            const station = stationSelect.value;
            if (station) params.set('station', station);
            updateURL(params);
        }
        state.currentDownload = null;
        updatePageTitle();
    });

    btnPlay.addEventListener('click', togglePlay);
    btnBack.addEventListener('click', () => {
        audioPlayer.currentTime = Math.max(0, audioPlayer.currentTime - 10);
    });
    btnFwd.addEventListener('click', () => {
        audioPlayer.currentTime = Math.min(audioPlayer.duration || 0, audioPlayer.currentTime + 30);
    });

    // Pause reels while seek buttons are held down
    for (const btn of [btnBack, btnFwd]) {
        for (const evt of ['mousedown', 'touchstart']) {
            btn.addEventListener(evt, () => stopReels());
        }
        for (const evt of ['mouseup', 'mouseleave', 'touchend']) {
            btn.addEventListener(evt, () => { if (state.isPlaying) startReels(); });
        }
    }
    btnHelp.addEventListener('click', () => {
        aboutModal.classList.remove('hidden');
    });

    audioPlayer.addEventListener('timeupdate', () => {
        const progress = (audioPlayer.currentTime / audioPlayer.duration) * 100;
        progressBar.value = progress || 0;
        timeCurrent.textContent = formatTime(audioPlayer.currentTime);
    });

    audioPlayer.addEventListener('loadedmetadata', () => {
        timeTotal.textContent = formatTime(audioPlayer.duration);
    });

    audioPlayer.addEventListener('error', (_e) => {
        const error = audioPlayer.error;
        debugError('Audio error:', error?.code, error?.message);
        debugAlert('Audio error: ' + (error?.message || 'Unknown error'));
    });

    audioPlayer.addEventListener('ended', () => {
        state.isPlaying = false;
        updatePlayButton();
        stopReels();
        // Auto-play next
        playNext();
    });

    audioPlayer.addEventListener('play', () => {
        state.isPlaying = true;
        updatePlayButton();
        startReels();
    });

    audioPlayer.addEventListener('pause', () => {
        state.isPlaying = false;
        updatePlayButton();
        stopReels();
    });

    // Pause reels while scrubbing the progress bar
    for (const evt of ['mousedown', 'touchstart']) {
        progressBar.addEventListener(evt, () => stopReels());
    }
    progressBar.addEventListener('input', (e) => {
        if (audioPlayer.duration) {
            audioPlayer.currentTime = (e.target.value / 100) * audioPlayer.duration;
        }
    });
    for (const evt of ['mouseup', 'touchend', 'change']) {
        progressBar.addEventListener(evt, () => { if (state.isPlaying) startReels(); });
    }

    // Handle browser back/forward navigation
    window.addEventListener('popstate', async () => {
        await applyURLState();
    });

    // Downloads page event listeners
    btnRecord.addEventListener('click', () => showPage('downloads'));
    backBtn.addEventListener('click', () => showPage('main'));

    // Download section - station change loads all shows
    dlStationSelect.addEventListener('change', async (e) => {
        const callSign = e.target.value;
        if (callSign) {
            await loadAllShows(callSign);
            renderAllShowsDropdown(dlShowSelect);
        } else {
            state.allShows = [];
            dlShowSelect.textContent = '';
            const opt = document.createElement('option');
            opt.value = '';
            opt.textContent = 'Select show...';
            dlShowSelect.appendChild(opt);
            dlShowSelect.disabled = true;
        }
    });

    // Action buttons
    downloadBtn.addEventListener('click', queueDownload);
    scheduleBtn.addEventListener('click', createSchedule);
}

// Start the app
init();
