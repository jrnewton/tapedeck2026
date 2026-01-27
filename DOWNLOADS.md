This document outlines the changes needed to the "download" function of the tapedeck app.

# Git
Pls create a new branch and perform all work on said branch.  This document has items marked with numbers "xx)".
Each number will ideally be its own self-contained commit.

# Planning
Each item "xx)" will have a plan document that includes a mockup (if appropriate), example API calls, proposed tests, etc.

# Backend changes
The CLI and server run in a Docker container which has no native ability to schedule download
jobs, that is put on the CLI user to utilize the host OS (linux) cron system.

1) [DONE] The server will be changed to include native (Golang) scheduling system so that it can
handle "schedule-download" CLI requests on its own.

# CLI changes
2) [DONE] "tapedeck-cli schedule-download" will need to connect to the running server and make a
request to schedule a download. The server will respond with status and perhaps an ID.

3) [DONE] A new CLI function "list-schedules" will be created to show all currently scheduled downloads,
their last run, last status and next run.

"tapedeck-cli download-show" should remain untouched, as it is a one-shot synchronous download
function. [DONE - validation added for station/show params]

7) [DONE] A new CLI function "delete-schedule" will be created to remove a scheduled download by ID.

# UI changes

4) In the audio player component, a new "record" button will be introduced. It will take the user to a page with several functions:

    4a) Download single episode, either "latest" or by picking a date. There will need to be an area of this page section showing status of "adhoc" downloads

    4b) schedule an episode download. This requires selecting a station and then selecting a show. the show list must be ALL shows.  The current shows widget displays shows with downloads already. that widget (used on main page) will remain the same. Perhaps an API change or filter change will be needed here. There will need to be an area of this page section showing all scheduled downloads, last run, last status and next run.

5) There will not be an audio player displayed at the bottom of this page.

6) This new "download" page will always be accessible via the record button and it will allow new downloads to be setup but also provide status on current activity.
