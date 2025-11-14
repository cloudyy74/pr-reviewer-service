create table if not exists teams (
    name varchar(64) primary key not null
);

create table if not exists users (
    id varchar(64) primary key not null,
    username varchar(64) not null,
    team_name varchar(64) references teams(name) on delete set null,
    is_active boolean not null default true
);

create index if not exists users_team_name_is_active_idx
    on users(team_name, is_active);