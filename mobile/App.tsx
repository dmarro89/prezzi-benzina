import { useCallback, useEffect, useMemo, useState } from 'react';
import Constants from 'expo-constants';
import * as Location from 'expo-location';
import {
  ActivityIndicator,
  Alert,
  FlatList,
  Linking,
  Platform,
  Pressable,
  RefreshControl,
  SafeAreaView,
  StatusBar,
  StyleSheet,
  Text,
  View,
} from 'react-native';

type Fuel = 'benzina' | 'gasolio' | 'gpl' | 'metano';
type Sort = 'price' | 'distance';
type Service = 'self' | 'served';
type Radius = 2 | 5 | 10;

type Price = { fuel: string; value: number; unit: string; service: string; updatedAt?: string };
type Station = {
  id: number;
  brand: string;
  name: string;
  address: string;
  city: string;
  province: string;
  location: { lat: number; lng: number };
  distanceKm: number;
  price: Price;
  prices?: Price[];
};

type NearbyResponse = { dataSource: string; liveData?: boolean; count: number; stations: Station[] };

function resolveApiUrl() {
  const configured = process.env.EXPO_PUBLIC_API_URL?.trim();
  if (configured) return configured.replace(/\/+$/, '');

  const hostUri = Constants.expoConfig?.hostUri;
  if (__DEV__ && hostUri) {
    try {
      const parsed = new URL(hostUri.includes('://') ? hostUri : `http://${hostUri}`);
      const host = parsed.hostname;
      const isLocalHost =
        host === 'localhost' ||
        host.endsWith('.local') ||
        /^10\./.test(host) ||
        /^192\.168\./.test(host) ||
        /^172\.(1[6-9]|2\d|3[01])\./.test(host) ||
        /^169\.254\./.test(host);
      if (isLocalHost) return `http://${host}:8080`;
    } catch {
      // Fall through to localhost for simulators or explicit env configuration.
    }
  }

  return 'http://localhost:8080';
}

const API_URL = resolveApiUrl();
const fuels: { key: Fuel; label: string }[] = [
  { key: 'benzina', label: 'Benzina' },
  { key: 'gasolio', label: 'Diesel' },
  { key: 'gpl', label: 'GPL' },
  { key: 'metano', label: 'Metano' },
];
const radii: Radius[] = [2, 5, 10];

export default function App() {
  const [fuel, setFuel] = useState<Fuel>('benzina');
  const [service, setService] = useState<Service>('self');
  const [radius, setRadius] = useState<Radius>(5);
  const [sort, setSort] = useState<Sort>('price');
  const [stations, setStations] = useState<Station[]>([]);
  const [coords, setCoords] = useState<{ lat: number; lng: number } | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const findLocation = useCallback(async () => {
    try {
      const permission = await Location.requestForegroundPermissionsAsync();
      if (permission.status !== 'granted') throw new Error('Permesso posizione non concesso');

      const last = await Location.getLastKnownPositionAsync({ maxAge: 10 * 60_000, requiredAccuracy: 3000 });
      const current = last ?? (await Location.getCurrentPositionAsync({ accuracy: Location.Accuracy.Balanced }));
      const next = { lat: current.coords.latitude, lng: current.coords.longitude };
      setCoords(next);
      return next;
    } catch (e) {
      const message = e instanceof Error ? e.message : 'errore sconosciuto';
      throw new Error(`Localizzazione: ${message}`);
    }
  }, []);

  const load = useCallback(async (isRefresh = false) => {
    try {
      isRefresh ? setRefreshing(true) : setLoading(true);
      setError(null);
      const location = coords ?? (await findLocation());
      const params = new URLSearchParams({
        lat: String(location.lat),
        lng: String(location.lng),
        radiusKm: String(radius),
        fuel,
        service,
        sort,
        limit: '100',
      });
      const controller = new AbortController();
      const timeout = setTimeout(() => controller.abort(), 10_000);
      let response: Response;
      try {
        response = await fetch(`${API_URL}/v1/stations/nearby?${params}`, { signal: controller.signal });
      } catch (e) {
        const message = e instanceof Error ? e.message : 'errore sconosciuto';
        if (controller.signal.aborted) {
          throw new Error(`API timeout dopo 10s (${API_URL})`);
        }
        throw new Error(`API non raggiungibile (${API_URL}): ${message}`);
      } finally {
        clearTimeout(timeout);
      }
      if (!response.ok) {
        const body = await response.text();
        throw new Error(`API ${response.status}: ${body}`);
      }
      const data: NearbyResponse = await response.json();
      setStations(data.stations);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Errore sconosciuto');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, [coords, findLocation, fuel, service, radius, sort]);

  useEffect(() => { void load(); }, [fuel, service, radius, sort]);

  const cheapest = useMemo(() => stations[0]?.price.value, [stations]);

  return (
    <SafeAreaView style={styles.safe}>
      <StatusBar barStyle="light-content" />
      <FlatList
        data={stations}
        keyExtractor={(item) => String(item.id)}
        refreshControl={<RefreshControl refreshing={refreshing} onRefresh={() => void load(true)} tintColor="#8CF0B0" />}
        contentContainerStyle={styles.content}
        ListHeaderComponent={
          <View>
            <View style={styles.topline}>
              <View>
                <Text style={styles.locationLabel}>●  La tua posizione</Text>
                <Text style={styles.locationDetail}>{coords ? `Distributori entro ${radius} km` : 'Localizzazione…'}</Text>
              </View>
              <View style={styles.liveBadge}><Text style={styles.liveText}>MIMIT</Text></View>
            </View>

            <Text style={styles.title}>Il prezzo migliore,{'\n'}più vicino.</Text>
            <Text style={styles.subtitle}>Prezzi ufficiali dei distributori intorno a te.</Text>

            <View style={styles.chips}>
              {fuels.map((item) => (
                <Pressable key={item.key} onPress={() => setFuel(item.key)} style={[styles.chip, fuel === item.key && styles.chipActive]}>
                  <Text style={[styles.chipText, fuel === item.key && styles.chipTextActive]}>{item.label}</Text>
                </Pressable>
              ))}
            </View>

            <View style={styles.filters}>
              <View style={styles.filterBlock}>
                <Text style={styles.filterLabel}>SERVIZIO</Text>
                <View style={styles.segmented}>
                  <Pressable onPress={() => setService('self')} style={[styles.segment, service === 'self' && styles.segmentActive]}>
                    <Text style={[styles.segmentText, service === 'self' && styles.segmentTextActive]}>Self</Text>
                  </Pressable>
                  <Pressable onPress={() => setService('served')} style={[styles.segment, service === 'served' && styles.segmentActive]}>
                    <Text style={[styles.segmentText, service === 'served' && styles.segmentTextActive]}>Servito</Text>
                  </Pressable>
                </View>
              </View>

              <View style={[styles.filterBlock, styles.radiusBlock]}>
                <Text style={styles.filterLabel}>RAGGIO</Text>
                <View style={styles.segmented}>
                  {radii.map((item) => (
                    <Pressable key={item} onPress={() => setRadius(item)} style={[styles.radiusSegment, radius === item && styles.segmentActive]}>
                      <Text style={[styles.segmentText, radius === item && styles.segmentTextActive]}>{item} km</Text>
                    </Pressable>
                  ))}
                </View>
              </View>
            </View>

            <View style={styles.sectionBar}>
              <View>
                <Text style={styles.sectionTitle}>VICINO A TE</Text>
                {cheapest ? <Text style={styles.sectionCaption}>Da {formatPrice(cheapest)} €/L · {service === 'self' ? 'Self' : 'Servito'}</Text> : null}
              </View>
              <Pressable onPress={() => setSort((s) => (s === 'price' ? 'distance' : 'price'))} style={styles.sortButton}>
                <Text style={styles.sortText}>{sort === 'price' ? 'Prezzo ↑' : 'Distanza ↑'}</Text>
              </Pressable>
            </View>

            {loading ? <View style={styles.center}><ActivityIndicator color="#8CF0B0" /><Text style={styles.muted}>Carico i prezzi MIMIT…</Text></View> : null}
            {error ? <View style={styles.error}><Text style={styles.errorTitle}>Non riesco a caricare i distributori</Text><Text style={styles.errorText}>{error}</Text><Pressable style={styles.retry} onPress={() => void load()}><Text style={styles.retryText}>Riprova</Text></Pressable></View> : null}
          </View>
        }
        renderItem={({ item, index }) => <StationCard station={item} best={sort === 'price' && index === 0} />}
        ListEmptyComponent={!loading && !error ? <Text style={styles.empty}>Nessun distributore trovato con questi filtri.</Text> : null}
        ListFooterComponent={<Text style={styles.footer}>Fonte: MIMIT · Open Data carburanti</Text>}
      />
    </SafeAreaView>
  );
}

function StationCard({ station, best }: { station: Station; best: boolean }) {
  const navigate = () => {
    const { lat, lng } = station.location;
    if (!Number.isFinite(lat) || !Number.isFinite(lng) || lat < -90 || lat > 90 || lng < -180 || lng > 180) {
      Alert.alert('Posizione non disponibile', 'Le coordinate di questo distributore non sono valide.');
      return;
    }

    const destination = encodeURIComponent(`${lat},${lng}`);
    const url = Platform.OS === 'ios'
      ? `https://maps.apple.com/directions?destination=${destination}`
      : `https://www.google.com/maps/dir/?api=1&destination=${destination}&travelmode=driving`;
    void Linking.openURL(url);
  };

  const name = (station.name || station.brand || 'Distributore').trim();
  const brand = station.brand?.trim() ?? '';
  const showBrand = brand !== '' && brand.toLocaleLowerCase('it-IT') !== name.toLocaleLowerCase('it-IT');
  const logoLabel = brand || name;
  const address = station.address || 'Indirizzo non disponibile';
  const locality = station.city
    ? `${station.city}${station.province ? ` (${station.province})` : ''}`
    : station.province;

  return (
    <Pressable onPress={navigate} style={({ pressed }) => [styles.card, pressed && styles.cardPressed]}>
      <View style={styles.logo}><Text style={styles.logoText}>{logoLabel.slice(0, 2).toUpperCase()}</Text></View>
      <View style={styles.cardBody}>
        <View style={styles.cardTitleRow}>
          <Text numberOfLines={1} style={styles.stationName}>{name}</Text>
          {best ? <View style={styles.bestBadge}><Text style={styles.bestText}>MIGLIORE</Text></View> : null}
        </View>
        {showBrand ? <Text numberOfLines={1} style={styles.brandLabel}>{brand}</Text> : null}
        <Text numberOfLines={1} style={styles.address}>{address}</Text>
        {locality ? <Text numberOfLines={1} style={styles.city}>{locality}</Text> : null}
        <Text style={styles.meta}>{station.distanceKm.toFixed(1).replace('.', ',')} km</Text>
      </View>
      <PriceRow price={station.price} />
    </Pressable>
  );
}

function PriceRow({ price }: { price: Price }) {
  const unit = price.unit === 'EUR/kg' ? '€/kg' : '€/L';
  const service = price.service === 'self' ? 'SELF' : 'SERVITO';
  const updated = price.updatedAt ? shortUpdated(price.updatedAt).replace('agg. ', '') : null;
  return (
    <View style={styles.priceRow}>
      <Text style={styles.serviceLabel}>{service}{updated ? ` · ${updated}` : ''}</Text>
      <View style={styles.priceValueRow}>
        <Text style={styles.price}>{formatPrice(price.value)}</Text>
        <Text style={styles.unit}>{unit}</Text>
      </View>
    </View>
  );
}

function formatPrice(value: number) { return value.toFixed(3).replace('.', ','); }
function shortUpdated(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return 'aggiornato';

  const now = new Date();
  const dateDay = Date.UTC(date.getFullYear(), date.getMonth(), date.getDate());
  const today = Date.UTC(now.getFullYear(), now.getMonth(), now.getDate());
  const dayDiff = Math.round((today - dateDay) / 86_400_000);
  const time = date.toLocaleTimeString('it-IT', { hour: '2-digit', minute: '2-digit' });

  if (dayDiff === 0) return `agg. oggi ${time}`;
  if (dayDiff === 1) return `agg. ieri ${time}`;

  const day = date.toLocaleDateString('it-IT', { day: '2-digit', month: '2-digit' });
  return `agg. ${day} ${time}`;
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: '#0B0C0C' },
  content: { paddingHorizontal: 18, paddingTop: 12, paddingBottom: 28 },
  topline: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', marginTop: 8 },
  locationLabel: { color: '#F5F6F5', fontSize: 15, fontWeight: '700' },
  locationDetail: { color: '#7F8582', fontSize: 12, marginTop: 4 },
  liveBadge: { borderWidth: 1, borderColor: '#2A302D', borderRadius: 999, paddingVertical: 8, paddingHorizontal: 12, backgroundColor: '#111412' },
  liveText: { color: '#8CF0B0', fontSize: 11, letterSpacing: 1.2, fontWeight: '800' },
  title: { color: '#F4F5F4', fontSize: 42, lineHeight: 44, letterSpacing: -1.6, fontWeight: '800', marginTop: 46 },
  subtitle: { color: '#A5AAA7', fontSize: 17, lineHeight: 24, marginTop: 14, maxWidth: 340 },
  chips: { flexDirection: 'row', gap: 8, marginTop: 28, marginBottom: 24 },
  chip: { borderWidth: 1, borderColor: '#303532', borderRadius: 999, paddingVertical: 11, paddingHorizontal: 16, backgroundColor: '#101211' },
  chipActive: { backgroundColor: '#8CF0B0', borderColor: '#8CF0B0' },
  chipText: { color: '#AEB2B0', fontWeight: '700', fontSize: 13 },
  chipTextActive: { color: '#07120B' },
  filters: { marginBottom: 30, gap: 14 },
  filterBlock: { gap: 7 },
  radiusBlock: { marginTop: 1 },
  filterLabel: { color: '#666D68', fontSize: 9, fontWeight: '800', letterSpacing: 1.1 },
  segmented: { flexDirection: 'row', alignSelf: 'flex-start', backgroundColor: '#101211', borderRadius: 11, borderWidth: 1, borderColor: '#242824', overflow: 'hidden' },
  segment: { minWidth: 82, alignItems: 'center', paddingVertical: 9, paddingHorizontal: 14 },
  radiusSegment: { alignItems: 'center', paddingVertical: 9, paddingHorizontal: 11 },
  segmentActive: { backgroundColor: '#E9ECE9' },
  segmentText: { color: '#8A908C', fontSize: 11, fontWeight: '700' },
  segmentTextActive: { color: '#111411' },
  sectionBar: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-end', marginBottom: 12 },
  sectionTitle: { color: '#E8EAE8', fontSize: 12, fontWeight: '800', letterSpacing: 1.4 },
  sectionCaption: { color: '#7F8582', fontSize: 12, marginTop: 5 },
  sortButton: { paddingVertical: 9, paddingHorizontal: 11, borderRadius: 10, backgroundColor: '#151816' },
  sortText: { color: '#BEC2BF', fontSize: 12, fontWeight: '700' },
  center: { paddingVertical: 35, alignItems: 'center', gap: 12 },
  muted: { color: '#777E79', fontSize: 13 },
  card: { minHeight: 108, backgroundColor: '#151716', borderRadius: 18, padding: 13, marginBottom: 10, flexDirection: 'row', alignItems: 'center', borderWidth: 1, borderColor: '#202320' },
  cardPressed: { opacity: 0.7 },
  logo: { width: 50, height: 50, borderRadius: 14, backgroundColor: '#F0F2EF', justifyContent: 'center', alignItems: 'center', marginRight: 12 },
  logoText: { color: '#101210', fontWeight: '900', fontSize: 15 },
  cardBody: { flex: 1, minWidth: 0 },
  cardTitleRow: { flexDirection: 'row', alignItems: 'center', gap: 7 },
  stationName: { color: '#F2F3F2', fontSize: 16, fontWeight: '800', maxWidth: '72%' },
  brandLabel: { color: '#8A908C', fontSize: 10, fontWeight: '700', marginTop: 3, letterSpacing: .35 },
  bestBadge: { backgroundColor: '#1E3325', borderRadius: 5, paddingHorizontal: 5, paddingVertical: 3 },
  bestText: { color: '#8CF0B0', fontWeight: '900', fontSize: 8, letterSpacing: .7 },
  address: { color: '#9A9E9B', fontSize: 12, marginTop: 4 },
  city: { color: '#777D79', fontSize: 11, marginTop: 3 },
  meta: { color: '#696F6B', fontSize: 10.5, marginTop: 7 },
  priceRow: { alignItems: 'flex-end', marginLeft: 10 },
  serviceLabel: { color: '#737A75', fontSize: 7.5, fontWeight: '800', letterSpacing: .45, marginBottom: 1 },
  priceValueRow: { flexDirection: 'row', alignItems: 'baseline', gap: 3 },
  price: { color: '#8CF0B0', fontSize: 20, letterSpacing: -.6, fontWeight: '800' },
  unit: { color: '#8B918D', fontSize: 9 },
  error: { backgroundColor: '#1B1515', borderRadius: 16, padding: 16, marginBottom: 12, borderWidth: 1, borderColor: '#3A2424' },
  errorTitle: { color: '#F2EAEA', fontWeight: '800', fontSize: 14 },
  errorText: { color: '#BCAAAA', fontSize: 11, marginTop: 7 },
  retry: { alignSelf: 'flex-start', marginTop: 12, backgroundColor: '#EAECE9', borderRadius: 9, paddingHorizontal: 12, paddingVertical: 8 },
  retryText: { color: '#121412', fontWeight: '800', fontSize: 12 },
  empty: { color: '#838985', paddingVertical: 36, textAlign: 'center' },
  footer: { color: '#4F5551', fontSize: 10, textAlign: 'center', marginTop: 22 },
});
