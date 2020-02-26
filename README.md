# Inventory POC

## The Service
The inventory is a [Resgate](https://resgate.io/) service that allows Resgate clients to subscribe
to data in an abstract `Storage`.

### File storage
A File based `Storage` that uses directories and yaml-files to store data of arbitrary complexity. This storage supports
CRUD.

### Bolt storage
This `Storage` can contain Bolt targets defined in YAML-files using the
[Bolt Inventory 2](https://puppet.com/docs/bolt/latest/inventory_file_v2.html) file format.

The storage is read-only from Resgate's point of view but sensitive to changes in the file system. The reasons for this
design are:
1. The main reason to be able to update the inventory in the first place is for test purposes only. In a production
   scenario, the inventory will reflect data stored elsewhere. Tests can swap the underlying yaml files instead of using
   proper Resgate events to update their content.
2. It's inherently hard to make changes to a target and determine where in the group hierarchy that change should be
   stored.
3. The most likely scenario moving forward is either that files will be extracted from PuppetDB somehow or that
   a storage, similar to the `bolt.Storage` included in the POC, will access PuppetDB directly.

CRUD support can of course be added later, should the need arise.

## Run the examples

### Install and start resgate and NATS

Easiest way to get Resgate and NATS up and running is to use [install Docker](https://docs.docker.com/install/).
Then do:
```
docker network create res
docker run -d --name nats -p 4222:4222 --net res nats
docker run -d --name resgate -p 8080:8080 --net res resgateio/resgate --nats nats://nats:4222
```
Now cd into the _\<project root>/examples/targets_ directory (main.go relies on that this is the current working
directory) and start the sample service:
```
cd <project root>examples/targets
go run main.go
```
Something similar to this should be displayed on the console:
```
Client at: http://localhost:8084/
INFO[0000] Connecting to NATS server                    
INFO[0000] Starting service                             
INFO[0000] Listening for requests                       
```
This service will load and listen to the files found under _\<project root>/testdata/volatile/bolt_. There are two of
them named _realm_a.yaml_ and _realm_b.yaml_. These files are Bolt Inventory 2 files and the bolt storage will consider
them to be two different _realms_.

The service also starts a webclient on http://localhost:8084. Direct the browser to that URL and a sample web page that
connects to Resgate and displays the targets will be displayed.

Now the fun starts. Try editing the realm files (don't worry, they are recreated when the sample service starts). Add
new facts to groups, change some config, add a new target. Meanwhile, watch what happens on the webpage as the changes
are detected by the Bolt Storage and published to Resgate.
