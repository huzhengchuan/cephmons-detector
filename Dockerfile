FROM golang

COPY ceph-operator /

CMD ["/ceph-operator", "-logtostderr","-v","5"]
