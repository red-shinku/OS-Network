echo "you should use sudo to run complie"

clang -g -O2 -target bpf -I. -c use_maps.bpf.c -o use_maps.bpf.o

echo "gen use_maps.bpf.o"

bpftool gen skeleton use_maps.bpf.o name openat_counts > use_maps.skel.h

echo "gen use_maps.skel.h"

gcc -g -O2 -Wall use_maps.c -o count_openat -lbpf -lz -lelf

echo "gen final program: count_openat"
echo "finish."
echo "use sudo ./count_openat to run"
