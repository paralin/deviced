Planned Support for Networks
============================

This is a planned addition to deviced which would make it incompatible for anything before 1.12.

Docker now supports named networks, like Google Cloud networks or Amazon VPCs.

Each has its own IP range (locally of course) as well as a gateway, etc.

Deviced should allow the following:

 - Specify a network in a "networks" configuration block
 - This additional network would be created / updated if it doesn't already exist.
 - Deviced will inspect the "network" option for containers to see if the network exists. If not this is a condition where deviced would wait to start the container until the network is created.
 - Network changes should re-trigger container sync and image sync
 - 

User Story: ROS
===============

The primary use case for this is of course a network of ROS nodes containing at minimum a ros core.

Additional could be:

 - Highlevel controller
 - Devman (?)
 - Lowlevel mavlink controller
 - etc...

These can be in their own internal network. ROS_IP and ROS_HOSTNAME can be set to: ROS_IP=ip of container, ROS_HOSTNAME=hostname of container.

Use --net-alias=ALIAS to define network aliases always. Names are predictable but not stable with deviced.
