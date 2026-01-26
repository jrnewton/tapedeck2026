// Tapedeck Frontend Application

const state = {
    stations: [],
    shows: [],
    downloads: [],
    currentDownload: null,
    isPlaying: false
};

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
    await loadStations();
    setupEventListeners();
}

// API Functions
async function fetchJSON(url) {
    const response = await fetch(url);
    if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    return response.json();
}

async function loadStations() {
    try {
        state.stations = await fetchJSON('/api/stations');
        renderStations();
    } catch (error) {
        console.error('Failed to load stations:', error);
        state.stations = [];
    }
}

async function loadShows(callSign) {
    try {
        state.shows = await fetchJSON(`/api/stations/${callSign}/shows`);
        renderShows();
    } catch (error) {
        console.error('Failed to load shows:', error);
        state.shows = [];
    }
}

async function loadDownloads(showId) {
    try {
        state.downloads = await fetchJSON(`/api/shows/${showId}/downloads?status=completed`);
        renderDownloads();
    } catch (error) {
        console.error('Failed to load downloads:', error);
        state.downloads = [];
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

        spine.innerHTML = `
            <div class="tape-info">
                <div class="tape-date">${dateStr}</div>
                <div class="tape-show">${download.Station} - ${download.Show}</div>
            </div>
            <button class="tape-play-btn" title="Play">&#9654;</button>
        `;

        spine.addEventListener('click', () => playDownload(download));
        tapeList.appendChild(spine);
    });
}

// Playback Functions
function playDownload(download) {
    state.currentDownload = download;
    audioPlayer.src = `/api/audio/${download.ID}`;
    audioPlayer.load();
    audioPlayer.play();
    state.isPlaying = true;
    updateNowPlaying();
    updatePlayButton();
    startReels();
    renderDownloads(); // Update active state
}

function updateNowPlaying() {
    if (!state.currentDownload) {
        nowPlaying.textContent = 'No tape loaded';
        return;
    }

    const download = state.currentDownload;
    const date = new Date(download.ArchiveDate);
    const dateStr = date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' });
    nowPlaying.textContent = `${download.Station} - ${download.Show}\n${dateStr}`;
}

function updatePlayButton() {
    const icon = btnPlay.querySelector('.icon');
    icon.innerHTML = state.isPlaying ? '&#9616;&#9616;' : '&#9654;';
}

function togglePlay() {
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
        audioPlayer.play();
        state.isPlaying = true;
        startReels();
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
    stationSelect.addEventListener('change', async (e) => {
        const callSign = e.target.value;
        if (callSign) {
            await loadShows(callSign);
        } else {
            state.shows = [];
            renderShows();
        }
        state.downloads = [];
        tapeList.innerHTML = '<p class="empty-message">Select a show to view downloads</p>';
    });

    showSelect.addEventListener('change', async (e) => {
        const showId = e.target.value;
        if (showId) {
            await loadDownloads(showId);
        } else {
            state.downloads = [];
            tapeList.innerHTML = '<p class="empty-message">Select a show to view downloads</p>';
        }
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
}

// Start the app
init();
