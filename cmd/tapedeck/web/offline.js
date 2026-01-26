// Tapedeck Offline Storage Module
// IndexedDB wrapper for storing audio blobs offline

const DB_NAME = 'tapedeck-offline';
const DB_VERSION = 1;
const STORE_NAME = 'audio';

let db = null;

// Open/initialize the database
function openDB() {
    return new Promise((resolve, reject) => {
        if (db) {
            resolve(db);
            return;
        }

        const request = indexedDB.open(DB_NAME, DB_VERSION);

        request.onerror = () => reject(request.error);

        request.onsuccess = () => {
            db = request.result;
            resolve(db);
        };

        request.onupgradeneeded = (event) => {
            const database = event.target.result;
            if (!database.objectStoreNames.contains(STORE_NAME)) {
                database.createObjectStore(STORE_NAME, { keyPath: 'downloadId' });
            }
        };
    });
}

// Save audio blob with metadata
async function saveAudio(downloadId, metadata, blob) {
    const database = await openDB();
    return new Promise((resolve, reject) => {
        const transaction = database.transaction([STORE_NAME], 'readwrite');
        const store = transaction.objectStore(STORE_NAME);

        const record = {
            downloadId,
            metadata,
            blob,
            savedAt: new Date().toISOString()
        };

        const request = store.put(record);
        request.onerror = () => reject(request.error);
        request.onsuccess = () => resolve();
    });
}

// Retrieve audio blob by download ID
async function getAudio(downloadId) {
    const database = await openDB();
    return new Promise((resolve, reject) => {
        const transaction = database.transaction([STORE_NAME], 'readonly');
        const store = transaction.objectStore(STORE_NAME);

        const request = store.get(downloadId);
        request.onerror = () => reject(request.error);
        request.onsuccess = () => resolve(request.result || null);
    });
}

// Delete audio by download ID
async function deleteAudio(downloadId) {
    const database = await openDB();
    return new Promise((resolve, reject) => {
        const transaction = database.transaction([STORE_NAME], 'readwrite');
        const store = transaction.objectStore(STORE_NAME);

        const request = store.delete(downloadId);
        request.onerror = () => reject(request.error);
        request.onsuccess = () => resolve();
    });
}

// List all stored download IDs
async function listOfflineIds() {
    const database = await openDB();
    return new Promise((resolve, reject) => {
        const transaction = database.transaction([STORE_NAME], 'readonly');
        const store = transaction.objectStore(STORE_NAME);

        const request = store.getAllKeys();
        request.onerror = () => reject(request.error);
        request.onsuccess = () => resolve(request.result || []);
    });
}

// Export as global module
window.offlineStorage = {
    openDB,
    saveAudio,
    getAudio,
    deleteAudio,
    listOfflineIds
};
