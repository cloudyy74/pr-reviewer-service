import http from 'k6/http';
import { check, sleep, group } from 'k6';

export const options = {
  scenarios: {
    steady: {
      executor: 'constant-arrival-rate',
      rate: 5,
      timeUnit: '1s',
      duration: '5m',
      preAllocatedVUs: 5,
      maxVUs: 10,
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<300'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://host.docker.internal:8080';
const TEAM_COUNT = 20;
const USERS_PER_TEAM = 10;

function createTeam(teamIndex) {
  const teamName = `team-${teamIndex}`;
  const members = [];
  for (let i = 1; i <= USERS_PER_TEAM; i += 1) {
    members.push({
      user_id: `user-${teamIndex}-${i}`,
      username: `user_${teamIndex}_${i}`,
      is_active: true,
    });
  }
  const payload = JSON.stringify({
    team_name: teamName,
    members,
  });
  const res = http.post(`${BASE_URL}/team/add`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });
  check(res, { 'team seeded': (r) => r.status === 201 || r.status === 200 });
  return { teamName, userIds: members.map((m) => m.user_id) };
}

export function setup() {
  const teams = [];
  for (let i = 1; i <= TEAM_COUNT; i += 1) {
    teams.push(createTeam(i));
  }
  return {
    teams,
    allUsers: teams.flatMap((team) => team.userIds),
  };
}

export default function (data) {
  const { teams, allUsers } = data;
  const team = teams[(__VU + __ITER) % teams.length];
  const author = team.userIds[0];
  const reviewerTarget = team.userIds[1];
  const randomUser = allUsers.length > 0 ? allUsers[(__VU * __ITER) % allUsers.length] : null;

  group('pr-create', () => {
    const randomSuffix = Math.random().toString(36).slice(2, 10);
    const payload = JSON.stringify({
      pull_request_id: `pr-${team.teamName}-${__VU}-${__ITER}-${Date.now()}-${randomSuffix}`,
      pull_request_name: 'Load test PR',
      author_id: author,
    });
    const res = http.post(`${BASE_URL}/pullRequest/create`, payload, {
      headers: { 'Content-Type': 'application/json' },
    });
    check(res, { 'pr created': (r) => r.status === 201 });
    if (res.status !== 201) {
      console.error(`pullRequest_create failed with ${res.status}: ${res.body}`)
    }

  });

  group('get-reviews', () => {
    const res = http.get(`${BASE_URL}/users/getReview?user_id=${reviewerTarget}`);
    check(res, { 'reviews ok': (r) => r.status === 200 });
  });

  group('stats', () => {
    const res = http.get(`${BASE_URL}/stats/assignments`);
    check(res, { 'stats ok': (r) => r.status === 200 });
  });

  group('set-active', () => {
    if (!randomUser) {
      return;
    }
    const payload = JSON.stringify({
      user_id: randomUser,
      is_active: true,
    });
    const res = http.post(`${BASE_URL}/users/setIsActive`, payload, {
      headers: { 'Content-Type': 'application/json' },
    });
    check(res, { 'set active ok': (r) => r.status === 200 });
  });

  sleep(0.2);
}
