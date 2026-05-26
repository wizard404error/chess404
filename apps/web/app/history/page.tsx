import HistoryRouteClient from './HistoryRouteClient';

function firstParam(value: string | string[] | undefined): string | null {
  if (Array.isArray(value)) {
    const first = value[0]?.trim();
    return first ? first : null;
  }
  const normalized = value?.trim();
  return normalized ? normalized : null;
}

export default async function HistoryPageRoute({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  return (
    <HistoryRouteClient
      replayMatchId={firstParam(params.replay)}
      guestId={firstParam(params.guest)}
    />
  );
}
