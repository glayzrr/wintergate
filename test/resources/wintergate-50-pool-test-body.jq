def upstream_host: "127.0.0.1";
def base_port: 18000;
def route_count: 50;
def dedicated_count: 5;

def endpoint:
  {
    path: "/**",
    method: "ALL",
    roles: []
  };

def pool_threshold:
  {
    hot: {
      rps: 1,
      "in-flight": 1
    },
    super: {
      rps: 50,
      "in-flight": 20
    }
  };

{
  global: {
    auth: {
      jwt_algorithm: "HS256",
      jwt_audience: "wintergate",
      jwt_clock_skew: "1m",
      jwt_issuer: "dummy-auth",
      jwt_secret: "dev-secret"
    }
  },
  routes: [
    range(1; route_count + 1) as $i |
    (
      {
        name: ("dummy-upstream-" + ($i | tostring)),
        host: upstream_host,
        port: (base_port + $i),
        endpoints: [endpoint]
      }
      +
      if $i <= dedicated_count then
        {
          threshold: pool_threshold
        }
      else
        {}
      end
    )
  ]
}
