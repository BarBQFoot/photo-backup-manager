# Prompt สำหรับ Google Stitch — Photo Backup Manager

คัดลอก Prompt ภาษาอังกฤษด้านล่างไปวางใน Google Stitch ได้เลย

---

Use the attached **FinalPhotoMover** program as the UX/UI reference for our project: keep its practical desktop layout, folder selection cards, image list/preview, selected-file move flow, progress panel, confirmation dialog, and compact history/integrity view. Redesign it with our project name, **Photo Backup Manager**, and add only the missing assignment features: a simple Gallery where the user can analyze a backed-up image into a Description, and a real Description search that returns images. Do not copy the reference app's placeholder search as final behavior.

Design a simple, minimal desktop UX/UI for a student project built with Wails and Nuxt for Windows and macOS. Keep the number of screens and components small; do not create separate screens for minor states or settings.

## Product purpose

Users choose a source folder or removable drive, choose a destination folder, scan and review image files, move selected files to the destination, track progress and results, check backup history and missing files, browse backed-up images, request an AI-generated image description, and search those descriptions to find matching images.

## Visual direction

- Minimal, calm, practical, and easy to understand for first-time users.
- Use a light neutral background, white surfaces, subtle borders, restrained shadows, and one muted blue or teal accent color.
- Use clear typography, generous but efficient spacing, simple line icons, and consistent status badges.
- Keep the layout desktop-first and responsive to common laptop widths.
- Prioritize readable file paths, filenames, image thumbnails, progress, and error messages.
- Avoid decorative gradients, oversized hero areas, excessive cards, charts without real data, and dense technical dashboards.
- Use realistic sample content only as visual design examples; label any sample data as demo content.

## Application structure

Use a persistent left sidebar and a main content area. Keep a compact overview on the main Move screen instead of adding a separate dashboard. The navigation items are:

1. Move Photos
2. Gallery (includes search)
3. History & Check

Show the currently selected drive/folder near the bottom of the sidebar. Do not add Settings, account/profile, cloud sync, tags, categories, tag cloud, advanced filters, or other product sections.

## Screens to design

### 1. Move Photos (main screen)

Use the reference app's main layout. At the top show source and destination folder cards with native-folder-picker style buttons and selected paths. Below them show a few compact counts for source images, destination images, backup records, and missing files. The source image list should support thumbnail preview and selection. Keep display modes simple (thumbnail grid and list are enough).

Design this as one straightforward page or a small sequence of steps (do not make a separate page for every step):

**Source → Destination → Scan → Review → Move → Progress → Result**

- Let users pick the source and destination folders using clear folder-picker buttons and show the chosen paths.
- Scan and show the image files found in a simple list with filename and duplicate indication; allow selecting files or all files.
- Provide one clear action to start moving the selected files. Show a compact progress indicator while it runs.
- At completion, show a short summary of moved, skipped, and failed files and elapsed time.

Do not imply that a failed move deletes the source file. Do not show a Changed-files status because it is not part of the current contract.

### 2. Gallery

Show backed-up images in a simple thumbnail grid. Selecting an image can open a lightweight detail panel with the image, filename, path, and description. Provide an Analyze action only when an image has no description. Mark missing files with a short warning and do not show a fake preview. Avoid designing many separate status variants.

### 3. History & Check

Follow the reference app's compact history table. Show past moved files for the selected destination with filename, path, size, status, and time. Include one integrity-check action that marks missing destination files and a clearly confirmed delete action for a destination file. Avoid filters and complicated history-management controls.

At the top of Gallery, place a compact text field for Description search limited to the currently selected destination. When the field is empty, show the regular folder image grid; when text is entered, show matching real images with filenames and a short description. Keep search and results on the Gallery page. Do not add tag search, tag cloud, categories, or advanced filters.

## Shared states and interaction details

- Design only the essential empty, working, and completed/error feedback within each main screen; do not create a screen for every state.
- Use short text labels for important statuses; do not rely on color alone.
- Missing files must be clearly marked and must not appear previewable.
- Use one concise confirmation before moving files.
- Keep controls accessible and error messages understandable to first-time users.

## Technical and scope constraints

- The app is a local desktop application: Nuxt UI calls Go through Wails. The frontend does not directly access SQLite or the filesystem.
- Real image files live on the selected drive/destination. SQLite stores metadata, full paths, AI descriptions, and backup history only; never design image storage in the database.
- The selected first-version scope is Description-only search using Full Path matching. Do not design Tags, Tag Cloud, Category, Hash identity, Changed/Renamed detection, cloud sync, phone-specific import protocols, or advanced search filters.
- AI analysis is user-triggered. Communicate clearly if AI analysis is unavailable; do not fabricate descriptions or search results.

Create a consistent, small set of connected screens based on the reference app. Preserve its understandable file-moving workflow and add only Gallery analysis and Description search within that Gallery. The result should be simple enough for a four-person student team with a short deadline to implement.

---

## ไฟล์บริบทที่แนบให้ Stitch

แนบไฟล์เหล่านี้พร้อม Prompt หาก Stitch รองรับการอัปโหลดเอกสาร:

1. `REFERENCE_APP_AND_ADDITIONS.md` — ฟีเจอร์ที่อ้างอิงจาก FinalPhotoMover และส่วนที่เพิ่ม
2. `05-UI-DESIGN.md` — โครงหน้าจอและข้อมูลที่ต้องแสดง
3. `03-UI-SPEC.md` — หน้าที่และสถานะของแต่ละหน้า
4. `04-FLOW-SPEC.md` — ลำดับ Backup, Gallery และ AI

หากแนบได้เพียงไฟล์เดียว ให้ใช้ Prompt นี้และแนบ `05-UI-DESIGN.md`
