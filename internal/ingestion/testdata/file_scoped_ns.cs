using System;

namespace Azimuth.Handlers;

public class RequestHandler
{
    public string HandlerId { get; set; }

    public string Handle(string request)
    {
        return request.ToUpper();
    }

    public bool CanHandle(string request)
    {
        return !string.IsNullOrEmpty(request);
    }
}
