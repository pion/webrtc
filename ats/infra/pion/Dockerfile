FROM golang:bullseye
RUN apt-get update
RUN apt-get install -y git bash openssh-server
COPY ./ats/infra/pion/ssh_config /etc/ssh/
RUN /etc/init.d/ssh start
# pion user with pion password to be used by clients' ssh connections
RUN useradd -s /bin/bash -d /home/pion -M -p '$y$j9T$6LsNr6MtK4Nt6NeECphjP1$Try0q9dVUdZGihzzxJZ0soO09wQhseYzyB/E2Jf4tO8' pion
RUN mkdir /home/pion
RUN chown pion /home/pion
CMD /etc/init.d/ssh start && tail -f /dev/null
