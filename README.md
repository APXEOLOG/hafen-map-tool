# hafen-map-tool
This is command line tool to merge minimaps from custom clients in Hafen (http://www.havenandhearth.com/)

Arguments:

* ```-d <folder>```: define input folder
* ```-z <session>```: create zoom layers for specific ```<session>``` and place them into "zoommap" folder
* ```-c```: remove all non-standard maps (size != 100x100)


Default folder is "sessions", default behavior is "cross-merge" (program tries to merge sessions with each other)

Usage example:

```map-merger -d maps``` - this will try to merge all sessions inside "maps"  

```map-merger -z "2015-08-30 12.47.06"``` - this will generate zoom layers from "sessions/2015-08-30 12.47.06"

```map-merger -d maps -z "2015-08-30 12.47.06"``` - this will generate zoom layers from "maps/2015-08-30 12.47.06"


HTML Viewers:

* sessions-viewer.html - Open this file after cross-merging sessions, it will show all results (you can select session in the session-box)
* map.html - This file will show result of the zoom process.
