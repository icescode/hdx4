# HDX-AGAR IPC → GUI FLOW
**flow-socket.md**

Dokumen ini menjelaskan **pemetaan final antara IPC hdx-agar dan aksi GUI**.
Ditulis ringkas, deterministik, dan siap dipakai sebagai referensi implementasi GUI desktop (FLTK / GTK / wxWidgets / Qt *optional*).

---

## 1. Koneksi Dasar

### Socket
- Path: `/tmp/hdx-agar.sock`
- Tipe: UNIX domain socket
- Model: **single owner, multi observer**

### Startup GUI
```
connect socket
WHOAMI
```

**Response**
- `OWNER` → GUI aktif (control enabled)
- `OBSERVER` → GUI read-only

---

## 2. Load Data Awal

### 2.1 List Volume (Album)
```
LIST-VOLUME
```

**Response**
```json
[
  { "index":0, "album":"Romantic Jazz 1988", "artist":"...", "total_tracks":9 },
  { "index":1, "album":"Vocal Jazz 2012", "artist":"...", "total_tracks":3 }
]
```

**GUI Action**
- Populate album list (left panel / grid)
- Simpan `volume_index`

---

### 2.2 List Track dalam Volume
```
LIST-HDXV <volume_index>
```

**GUI Action**
- Populate track table
- Track index **0-based**

---

## 3. Playback Control

### 3.1 Play Album
```
PLAY-VOLUME <volume_index>
```

GUI:
- Highlight album
- Auto select track 0

---

### 3.2 Play Track Spesifik
```
PLAY-TRACK <volume_index> <track_index>
```

GUI:
- Highlight track
- Update “Now Playing”

---

### 3.3 Next Track
```
NEXT
```

GUI:
- Advance track selection

---

### 3.4 Pause / Resume
```
PAUSE
RESUME
```

GUI:
- Toggle icon
- Jangan menebak state → dengarkan EVENT / STATUS

---

### 3.5 Stop
```
STOP
```

GUI:
- Clear playing indicator
- Track selection boleh tetap

---

## 4. Library Management

### 4.1 Add Volume
```
ADD-VOLUME /absolute/path/to/file.hdxv
```

GUI:
- File picker
- Setelah OK → refresh `LIST-VOLUME`

Error yang harus ditangani:
- `ERR DUPLICATE`
- `ERR INVALID_MAGIC`
- `ERR AUTH_FAILED`

---

### 4.2 Remove Volume
```
REMOVE-VOLUME <volume_index>
```

GUI:
- Confirmation dialog
- Refresh list jika OK

Error:
- `ERR VOLUME_IN_USE`
- `ERR INDEX`

---

## 5. STATUS & EVENT (Realtime)

### 5.1 Event Push (Preferred)
Engine akan mengirim EVENT tanpa polling berat.

#### STATUS
```json
EVENT {
  "type":"STATUS",
  "playing":true,
  "paused":false,
  "volume_index":1,
  "track_index":2
}
```

GUI:
- Update play/pause
- Sync highlight album & track

---

#### TRACK_CHANGED
```json
EVENT {
  "type":"TRACK_CHANGED",
  "volume_index":1,
  "track_index":2
}
```

GUI:
- Scroll & highlight track

---

#### STOPPED
```json
EVENT {
  "type":"STOPPED",
  "playing":false
}
```

GUI:
- Reset progress
- Disable pause button

---

### 5.2 STATUS Polling (Optional)
Jika GUI tidak memakai EVENT.

```
STATUS
```

Response:
```json
{
  "playing":true,
  "paused":false,
  "volume_index":1,
  "track_index":2
}
```

**Polling interval aman:** 500ms – 1000ms

---

## 6. Error → GUI Mapping

| IPC Error | GUI Behavior |
|---------|--------------|
| `ERR CONTROL_LOCKED` | Show “Player in use” |
| `ERR ARG` | Ignore / log |
| `ERR INDEX` | Refresh list |
| `ERR DUPLICATE` | Notify user |
| `ERR VOLUME_IN_USE` | Warning dialog |

---

## 7. Minimal GUI State Model

GUI hanya perlu menyimpan:

```
currentVolumeIndex
currentTrackIndex
isPlaying
isPaused
```

GUI **tidak perlu tahu**:
- Opus
- AES / crypto
- streaming detail
- audio timing presisi

Engine = single source of truth.

---

## 8. Kesimpulan

- IPC sudah stabil dan deterministic
- GUI hanya bertindak sebagai **controller + renderer**
- Tidak perlu modifikasi engine untuk UI
- Cocok untuk desktop low CPU / low memory

**Status:**
✔ IPC ready
✔ GUI can start
✔ No blocking technical debt
