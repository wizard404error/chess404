create table if not exists accounts (
    account_id text primary key,
    handle text not null unique,
    primary_guest_id text not null,
    linked_guest_ids text not null,
    rating integer not null default 1200,
    matches_played integer not null default 0,
    wins integer not null default 0,
    losses integer not null default 0,
    draws integer not null default 0,
    rating_history text not null default '[]',
    created_at text not null,
    last_seen_at text not null,
    last_active_at text,
    session_token text,
    session_expires_at text
);

create table if not exists account_guest_links (
    guest_id text primary key,
    account_id text not null references accounts(account_id) on delete cascade
);

create table if not exists account_finalized_matches (
    match_id text primary key,
    winner text not null
);

create table if not exists account_credentials (
    account_id text primary key references accounts(account_id) on delete cascade,
    email text not null unique,
    password_hash text not null,
    email_verified_at text
);

create table if not exists account_email_verifications (
    account_id text not null references accounts(account_id) on delete cascade,
    token text primary key,
    email text not null,
    expires_at text not null,
    created_at text not null,
    used_at text,
    updated_at text not null
);

create table if not exists account_password_resets (
    account_id text not null references accounts(account_id) on delete cascade,
    token text primary key,
    expires_at text not null,
    created_at text not null,
    used_at text,
    updated_at text not null
);

create table if not exists account_sessions (
    account_id text not null references accounts(account_id) on delete cascade,
    session_token text primary key,
    expires_at text not null,
    created_at text not null,
    last_seen_at text not null
);

create table if not exists guests (
    guest_id text primary key,
    display_name text not null,
    rating integer not null,
    matches_played integer not null,
    wins integer not null,
    losses integer not null,
    draws integer not null,
    created_at text not null,
    last_seen_at text not null,
    session_secret text,
    session_token text,
    session_expires_at text
);

create table if not exists finalized_matches (
    match_id text primary key,
    winner text not null,
    finalized_at text not null
);

create table if not exists archives (
    match_id text primary key,
    entry_json text not null,
    private_json text
);

create table if not exists direct_challenges (
    challenge_id text primary key,
    challenger_account_id text not null,
    target_account_id text not null,
    match_id text not null,
    mode_id text not null,
    clock_seconds integer not null,
    challenger_seat text not null,
    status text not null,
    created_at text not null,
    updated_at text not null
);

create table if not exists friend_requests (
    request_id text primary key,
    requester_account_id text not null,
    target_account_id text not null,
    status text not null,
    created_at text not null,
    updated_at text not null
);

create table if not exists friendships (
    friendship_id text primary key,
    low_account_id text not null,
    high_account_id text not null,
    created_at text not null
);

create table if not exists account_email_outbox (
    delivery_id text primary key,
    account_id text not null,
    email text not null,
    kind text not null,
    subject text not null,
    text_body text not null,
    html_body text not null,
    action_url text not null,
    status text not null,
    provider text not null default '',
    provider_message_id text not null default '',
    attempt_count integer not null default 0,
    last_attempt_at text null,
    next_attempt_at text null,
    delivered_at text null,
    failed_at text null,
    failure_reason text not null default '',
    created_at text not null,
    updated_at text not null
);

create table if not exists account_notifications (
    notification_id text primary key,
    account_id text not null,
    actor_account_id text not null,
    kind text not null,
    friend_request_id text,
    challenge_id text,
    match_id text,
    mode_id text not null,
    challenger_seat text,
    created_at text not null,
    updated_at text not null,
    read_at text
);

create table if not exists account_blocks (
    block_id text primary key,
    blocker_account_id text not null,
    target_account_id text not null,
    reason text not null,
    created_at text not null,
    updated_at text not null
);

create table if not exists player_reports (
    report_id text primary key,
    reporter_account_id text not null,
    target_account_id text not null,
    category text not null,
    details text not null,
    status text not null,
    reviewed_by_account_id text,
    reviewed_at text,
    resolution_note text not null default '',
    created_at text not null,
    updated_at text not null
);

create table if not exists moderation_actions (
    action_id text primary key,
    report_id text not null,
    moderator_account_id text not null,
    reporter_account_id text not null,
    target_account_id text not null,
    previous_status text not null,
    next_status text not null,
    action text not null,
    note text not null default '',
    created_at text not null
);

create table if not exists account_restrictions (
    account_id text primary key,
    restriction_id text not null,
    kind text not null,
    reason text not null default '',
    report_id text,
    applied_by_account_id text,
    created_at text not null,
    updated_at text not null
);

create table if not exists account_security_events (
    event_id text primary key,
    account_id text not null,
    kind text not null,
    detail text not null,
    created_at text not null
);

create table if not exists tickets (
    ticket_id text primary key,
    guest_id text not null,
    account_id text,
    display_name text,
    queue text not null,
    mode_id text,
    status text not null,
    rating integer not null,
    created_at text not null,
    updated_at text not null,
    matched_at text,
    matched_with text,
    seat_color text,
    opponent_name text,
    assigned_room text
);

create index if not exists accounts_last_seen_order_idx on accounts (last_seen_at desc, created_at desc, account_id asc);
create index if not exists account_sessions_account_idx on account_sessions (account_id, last_seen_at desc, created_at desc, session_token asc);
create index if not exists account_sessions_expires_idx on account_sessions (expires_at);
create index if not exists account_email_verifications_account_idx on account_email_verifications (account_id, created_at desc);
create index if not exists account_password_resets_account_idx on account_password_resets (account_id, created_at desc);
create index if not exists direct_challenges_challenger_idx on direct_challenges (challenger_account_id);
create index if not exists direct_challenges_target_idx on direct_challenges (target_account_id);
create index if not exists direct_challenges_status_idx on direct_challenges (status);
create index if not exists friend_requests_requester_idx on friend_requests (requester_account_id);
create index if not exists friend_requests_target_idx on friend_requests (target_account_id);
create index if not exists friendships_low_idx on friendships (low_account_id);
create index if not exists friendships_high_idx on friendships (high_account_id);
create index if not exists account_email_outbox_account_idx on account_email_outbox (account_id, updated_at desc, created_at desc, delivery_id asc);
create index if not exists account_email_outbox_status_idx on account_email_outbox (status, next_attempt_at asc, updated_at desc, created_at desc, delivery_id asc);
create index if not exists account_notifications_account_idx on account_notifications (account_id, updated_at desc, created_at desc);
create index if not exists account_notifications_actor_idx on account_notifications (actor_account_id);
create index if not exists account_notifications_read_idx on account_notifications (account_id, read_at);
create index if not exists account_blocks_blocker_idx on account_blocks (blocker_account_id);
create index if not exists account_blocks_target_idx on account_blocks (target_account_id);
create index if not exists player_reports_reporter_idx on player_reports (reporter_account_id);
create index if not exists player_reports_target_idx on player_reports (target_account_id);
create index if not exists player_reports_status_idx on player_reports (status);
create index if not exists moderation_actions_report_idx on moderation_actions (report_id);
create index if not exists moderation_actions_moderator_idx on moderation_actions (moderator_account_id);
create index if not exists account_restrictions_kind_idx on account_restrictions (kind);
create index if not exists account_security_events_account_idx on account_security_events (account_id, created_at desc, event_id asc);
