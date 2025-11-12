create table if not exists pull_requests (
    id uuid primary key not null default uuid_generate_v4(),
    title varchar(64) not null,
    author_id uuid not null references users(id) on delete cascade,
    status varchar(16) not null check (status in ('OPEN', 'MERGED')),
    reviewers_ids uuid[] check (cardinality(reviewers_ids) <= 2),
    need_more_reviewers boolean not null default false
);