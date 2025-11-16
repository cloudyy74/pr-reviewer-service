create table if not exists statuses (
    id serial primary key,
    name varchar(64) unique not null
);

create table if not exists pull_requests (
    id varchar(64) primary key not null,
    title varchar(256) not null,
    author_id varchar(64) not null references users(id) on delete cascade,
    status_id int not null references statuses(id),
    merged_at timestamp with time zone
);

create index if not exists pull_requests_status_id_idx
    on pull_requests(status_id);

create table if not exists pull_requests_reviewers (
    pull_request_id varchar(64) not null references pull_requests(id) on delete cascade,
    user_id varchar(64) not null references users(id) on delete cascade,
    primary key (pull_request_id, user_id)
); 

create index if not exists pull_requests_reviewers_user_id_idx
    on pull_requests_reviewers(user_id, pull_request_id);
