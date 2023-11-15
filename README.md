# Zima

PoC.

TODO: think about how we want to do roles properly. options:
 - a) outside of core (lib)
 - b) in core via `CreateRole`, `AddPermission`, etc.

 ^ consider checking, listing. do I want to be able to check/list resources I
have a certain permission to? Or happy to use lib to list by each role that
provides it. Also transparency and ease of understanding why a relation is there,
to do that it might require list of roles that provide a permission on each tuple.
