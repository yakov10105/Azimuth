namespace Azimuth.Services
{
    public class PaymentProcessor
    {
        public string ProcessorId { get; set; }

        public class PaymentConfig
        {
            public int TimeoutMs { get; set; }
            public string ApiKey { get; set; }
        }

        public Result Process(PaymentConfig config)
        {
            return new Result();
        }
    }
}
