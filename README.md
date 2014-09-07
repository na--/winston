Winston
=======

Winston is a simple tool used to download the metadata of .torrent files by their infohash (magnet link).


Installation
------------
```
go get github.com/na--/winston
```

Usage
-----
```
winston [options] infohash1 [infohash2 ...]
```

To see logs, use the -logtostderr flag and change the verbosity with -v 0 (less verbose) up to -v 5 (more verbose).

Possible options:
 * -h: Show the help message
 * -output_folder: Folder where you want to save the downloaded torrent metadata files [default="./tmp/"]
 * -v: Log verbosity, from 0 (less verbose) to 5 (most verbose) [default=0]
 * -logtostderr: Log to standard error instead of files [default=false]
 * -alsologtostderr: Also use stderr for log output as well as files [default=false]
 * -log_dir: If non-empty, write log files in this directory [default=""]
 * -stderrthreshold: logs at or above this threshold go to stderr [default=0]

Example
-------
```
winston -v=2 -logtostderr -output_folder="./" 4d753474429d817b80ff9e0c441ca660ec5d2450
```
This will download the torrent file for ubuntu-14.04-desktop-amd64 and save it in the current folder, while showing a fair amount of logs.


Project Status
--------------
Winston is functional but still needs major improvements and optimizations. As it is, it can be used as a demonstration or with small amounts of infohashes. Heavier workloads will be possible in the future (see the roadmap).


Roadmap
-------

1. Finish the simple command-line version
    - Optimize parallel downloads
    - Improve timeout handling
    - Implement better DHT processing (asking for more peers, better library usage, etc.)
    - Remove hardcoded constants and use flags and/or config file
    - Support magnet links
    - Support getting peers for regular torrent trackers, not just DHT
    - Support PEX
2. Create a simple web user interface
    - Add a persistent work mode that has a simple web interface
    - Allow users to interactively add new hashes via the interface
    - Show download progress and status for the added hashes
    - Add functionality for parsing the downloaded torrent metadata files
    - Add functionality for searching in the torrent metadata
3. Create a DHT-listening active search engine
    - Use multiple long-running DHT nodes for actively listening to the DHT network
    - Implement other parts of BEP09 and send known metadata to other nodes (help the network)
    - When one of the nodes finds out an unknown infoHash, winston will attempt to download it
4. Scrapers for augmenting the torrent infodata
    - Scrape normal http torrent directories
    - If new hashes are found, attempt to download the torrent metadata
    - Download other metadata from the torrent directories themselves and connect it with our own


Libraries
---------

You can use some of Winston's publicly exported library functions for your own projects:
* http://godoc.org/github.com/na--/winston/torrent/metadata
* http://godoc.org/github.com/na--/winston/torrent/peer

Credits
-------
Winston uses the following libraries:
 * https://github.com/nictuku/dht
 * https://github.com/golang/glog
 * https://github.com/jackpal/bencode-go

Also, Winston borrows quite a lot of ideas and some code from [Taipei-Torrent](https://github.com/jackpal/Taipei-Torrent) by jackpal

License
-------
Licensed under the [MIT License](http://opensource.org/licenses/MIT)
