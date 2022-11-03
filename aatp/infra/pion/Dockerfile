FROM golang:bullseye
RUN apt update
RUN apt install -y git bash openssh-server
COPY ./aatp/infra/pion/ssh_config /etc/ssh/
COPY ./aatp/infra/pion/docker_entry.sh /usr/local/bin
RUN /etc/init.d/ssh start
RUN useradd -s /bin/bash -d /home/pion -M -p '$y$j9T$6LsNr6MtK4Nt6NeECphjP1$Try0q9dVUdZGihzzxJZ0soO09wQhseYzyB/E2Jf4tO8' pion
WORKDIR /source
CMD go run .
