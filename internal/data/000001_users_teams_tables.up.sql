create table if not exists teams (
    name varchar(64) primary key not null
);

create table if not exists users (
    id varchar(64) primary key not null,
    name varchar(64) not null,
    team_name varchar(64) references teams(name) on delete set null,
    is_active boolean not null default true
);