create table if not exists statuses (
    id serial primary key,
    name varchar(64) unique not null
);

create index statuses_name_idx on statuses(name);

create table if not exists pull_requests (
    id varchar(64) primary key not null,
    title varchar(256) not null,
    author_id varchar(64) not null references users(id) on delete cascade,
    status_id int not null references statuses(id),
    need_more_reviewers boolean not null default false
);

create table if not exists pull_requests_reviewers (
    pull_request_id varchar(64) not null references pull_requests(id) on delete cascade,
    user_id varchar(64) not null references users(id) on delete cascade,
    primary key (pull_request_id, user_id)
);