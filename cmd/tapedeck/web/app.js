// Tapedeck Frontend Application - Mobile Version

const state = {
    stations: [],
    shows: [],
    downloads: [],
    currentDownload: null,
    isPlaying: false,
    offlineIds: new Set(),      // tracks which downloads are saved offline
    downloadingIds: new Set(),  // tracks downloads in progress
    debugMode: localStorage.getItem('debugMode') === 'true'
};

// Debug helper - only shows alerts when debug mode is enabled
function debugAlert(message) {
    console.log('[DEBUG]', message);
    if (state.debugMode) {
        alert(message);
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
    const station = params.get('station');
    const showId = params.get('show');
    const playId = params.get('play');

    if (station) {
        stationSelect.value = station;
        await loadShows(station);

        if (showId) {
            showSelect.value = showId;
            await loadDownloads(showId);

            if (playId) {
                const download = state.downloads.find(d => d.ID == playId);
                if (download) await loadDownloadWithoutPlay(download); // Load but don't autoplay
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
const btnStop = document.getElementById('btn-stop');
const btnPrev = document.getElementById('btn-prev');
const btnNext = document.getElementById('btn-next');
const progressBar = document.getElementById('progress-bar');
const timeCurrent = document.getElementById('time-current');
const timeTotal = document.getElementById('time-total');
const leftReel = document.querySelector('.left-reel');
const rightReel = document.querySelector('.right-reel');

// Initialize
async function init() {
    // Register service worker for offline app shell
    if ('serviceWorker' in navigator) {
        navigator.serviceWorker.register('/sw.js').catch((error) => {
            console.warn('Service worker registration failed:', error);
        });
    }

    // Load offline IDs from IndexedDB
    await loadOfflineIds();

    await loadStations();
    setupEventListeners();
    await applyURLState();
}

// Load offline download IDs from IndexedDB
async function loadOfflineIds() {
    try {
        const ids = await window.offlineStorage.listOfflineIds();
        state.offlineIds = new Set(ids);
    } catch (error) {
        console.warn('Failed to load offline IDs:', error);
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
    } catch (e) {
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
            state.shows = data;
            renderShows();
        };
    }
    if (url.match(/^\/api\/shows\/\d+\/downloads/)) {
        return (data) => {
            state.downloads = data;
            renderDownloads();
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
    } catch (e) {
        // localStorage might be full
    }
    return data;
}

async function loadStations() {
    try {
        state.stations = await fetchJSON('/api/stations');
        renderStations();
    } catch (error) {
        console.error('Failed to load stations:', error);
        state.stations = [];
        renderStations();
    }
}

async function loadShows(callSign) {
    try {
        state.shows = await fetchJSON(`/api/stations/${callSign}/shows`);
        renderShows();
    } catch (error) {
        console.error('Failed to load shows:', error);
        state.shows = [];
        renderShows();
    }
}

async function loadDownloads(showId) {
    try {
        state.downloads = await fetchJSON(`/api/shows/${showId}/downloads?status=completed`);
        renderDownloads();
    } catch (error) {
        console.error('Failed to load downloads:', error);
        state.downloads = [];
        renderDownloads();
    }
}

// Render Functions
function renderStations() {
    stationSelect.innerHTML = '<option value="">Station...</option>';
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
    if (state.downloads.length === 0) {
        tapeList.innerHTML = '<p class="empty-message">No completed downloads for this show</p>';
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
        });

        // Determine offline button state
        const isOffline = state.offlineIds.has(download.ID);
        const isDownloading = state.downloadingIds.has(download.ID);
        let btnClass = 'offline-btn';
        let btnContent = '\u2193'; // Down arrow
        if (isDownloading) {
            btnClass += ' downloading';
            btnContent = ''; // Spinner via CSS
        } else if (isOffline) {
            btnClass += ' saved';
            btnContent = '\u2713'; // Checkmark
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
            console.error('Failed to remove offline audio:', error);
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
        console.log(`Downloaded ${sizeMB}MB blob for ID ${download.ID}`);

        const metadata = {
            station: download.Station,
            show: download.Show,
            archiveDate: download.ArchiveDate
        };

        await window.offlineStorage.saveAudio(download.ID, metadata, blob);
        state.offlineIds.add(download.ID);
        console.log(`Saved to IndexedDB: ID ${download.ID}`);
    } catch (error) {
        console.error('Failed to save audio offline:', error);
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
                console.log('Playing from offline storage:', blobUrl);
                return blobUrl;
            }
            console.warn('Offline record found but no blob for ID:', download.ID);
        } catch (error) {
            console.warn('Failed to load offline audio, falling back to network:', error);
        }
    }
    console.log('Playing from network:', `/api/audio/${download.ID}`);
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
        console.error('Playback failed:', error.name, error.message);
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
    const dateStr = date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' });
    nowPlaying.textContent = `${download.Show} · ${dateStr}`;
}

function updatePlayButton() {
    const icon = btnPlay.querySelector('.icon');
    icon.innerHTML = state.isPlaying ? '&#9616;&#9616;' : '&#9654;';
}

async function togglePlay() {
    if (!state.currentDownload) {
        // If no download selected, play first one
        if (state.downloads.length > 0) {
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
            console.error('Playback failed:', error.name, error.message);
            debugAlert('Playback failed: ' + error.message);
        }
    }
    updatePlayButton();
}

function stop() {
    audioPlayer.pause();
    audioPlayer.currentTime = 0;
    state.isPlaying = false;
    updatePlayButton();
    stopReels();
}

function playNext() {
    if (!state.currentDownload || state.downloads.length === 0) return;

    const currentIndex = state.downloads.findIndex(d => d.ID === state.currentDownload.ID);
    const nextIndex = (currentIndex + 1) % state.downloads.length;
    playDownload(state.downloads[nextIndex]);
}

function playPrev() {
    if (!state.currentDownload || state.downloads.length === 0) return;

    const currentIndex = state.downloads.findIndex(d => d.ID === state.currentDownload.ID);
    const prevIndex = (currentIndex - 1 + state.downloads.length) % state.downloads.length;
    playDownload(state.downloads[prevIndex]);
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
        alert('Debug mode: ' + (state.debugMode ? 'ON' : 'OFF'));
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
        tapeList.textContent = '';
        const msg = document.createElement('p');
        msg.className = 'empty-message';
        msg.textContent = 'Select a show to view downloads';
        tapeList.appendChild(msg);
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
            tapeList.textContent = '';
            const msg = document.createElement('p');
            msg.className = 'empty-message';
            msg.textContent = 'Select a show to view downloads';
            tapeList.appendChild(msg);
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
    btnStop.addEventListener('click', stop);
    btnNext.addEventListener('click', playNext);
    btnPrev.addEventListener('click', playPrev);

    audioPlayer.addEventListener('timeupdate', () => {
        const progress = (audioPlayer.currentTime / audioPlayer.duration) * 100;
        progressBar.value = progress || 0;
        timeCurrent.textContent = formatTime(audioPlayer.currentTime);
    });

    audioPlayer.addEventListener('loadedmetadata', () => {
        timeTotal.textContent = formatTime(audioPlayer.duration);
    });

    audioPlayer.addEventListener('error', (e) => {
        const error = audioPlayer.error;
        console.error('Audio error:', error?.code, error?.message);
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

    progressBar.addEventListener('input', (e) => {
        if (audioPlayer.duration) {
            audioPlayer.currentTime = (e.target.value / 100) * audioPlayer.duration;
        }
    });

    // Handle browser back/forward navigation
    window.addEventListener('popstate', async () => {
        await applyURLState();
    });
}

// Start the app
init();
